// Package golang resolves Go architecture using compiler type information.
package golang

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
	analyzer "seamgraph/internal/analysis"
	"seamgraph/internal/config"
	"seamgraph/internal/ir"
)

type Adapter struct{}

func (Adapter) Analyze(ctx context.Context, project analyzer.Project, c config.Config) (ir.PartialSnapshot, error) {
	b, err := analyzer.NewBuilder(project.Root, c, "go")
	if err != nil {
		return ir.PartialSnapshot{}, err
	}
	patterns := []string{}
	for _, m := range c.Modules {
		if m.Language == "go" {
			for _, root := range m.Paths {
				patterns = append(patterns, "./"+root+"/...")
			}
		}
	}
	buildFlags := []string{"-mod=readonly"}
	if _, err := os.Stat(filepath.Join(b.Root, "vendor", "modules.txt")); err == nil {
		buildFlags = []string{"-mod=vendor"}
	}
	// Preserve Go toolchain selection and checksum verification for cached toolchains.
	// The disabled module proxy prevents fetching missing modules or toolchains.
	pkgs, err := packages.Load(&packages.Config{Context: ctx, Dir: b.Root, BuildFlags: buildFlags, Env: append(os.Environ(), "GOPROXY=off"), Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule}, patterns...)
	if err != nil {
		return b.Partial, err
	}
	if len(pkgs) == 0 {
		return b.Partial, fmt.Errorf("no go packages found")
	}
	var loadErr error
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			if loadErr == nil {
				loadErr = fmt.Errorf("go analysis: %s", e.Error())
			}
		}
	})
	if loadErr != nil {
		return b.Partial, loadErr
	}
	g := goAnalysis{b: b, objects: map[types.Object]string{}, symbolPackages: map[string]string{}, names: map[string]map[string][]string{}, named: map[string]*types.Named{}}
	g.collectDeclarations(pkgs)
	g.collectFields(pkgs)
	g.collectDefinitions(pkgs)
	if err := g.resolveSeams(c); err != nil {
		return b.Partial, err
	}
	g.collectReferences(pkgs, c)
	g.collectImplementations(pkgs)
	return b.Finish(), nil
}
func appendUnique(ids []string, id string) []string {
	for _, s := range ids {
		if s == id {
			return ids
		}
	}
	return append(ids, id)
}

type goAnalysis struct {
	b              *analyzer.Builder
	objects        map[types.Object]string
	symbolPackages map[string]string
	names          map[string]map[string][]string
	named          map[string]*types.Named
}

func (g *goAnalysis) evidence(p *packages.Package, pos, end token.Pos) ir.Evidence {
	a, z := p.Fset.Position(pos), p.Fset.Position(end)
	file, _ := g.b.Relative(a.Filename)
	return ir.Evidence{File: file, StartLine: a.Line, StartColumn: a.Column, EndLine: z.Line, EndColumn: z.Column, Confidence: "exact", Resolution: "resolved"}
}

func (g *goAnalysis) collectDeclarations(pkgs []*packages.Package) {
	for _, p := range pkgs {
		for ident, obj := range p.TypesInfo.Defs {
			g.collectDeclaration(p, ident, obj)
		}
	}
}

func (g *goAnalysis) collectFields(pkgs []*packages.Package) {
	// Struct fields are real cross-module references too, not just declarations.
	for _, p := range pkgs {
		declared := declaredStructs(p)
		for ident, obj := range p.TypesInfo.Defs {
			tn, ok := obj.(*types.TypeName)
			if !ok || !declared[tn] || g.objects[tn] == "" {
				continue
			}
			n, ok := types.Unalias(tn.Type()).(*types.Named)
			if !ok {
				continue
			}
			st, ok := n.Underlying().(*types.Struct)
			if !ok {
				continue
			}
			file := p.Fset.Position(ident.Pos()).Filename
			owner := g.b.Owner(file)
			rel, _ := g.b.Relative(file)
			for i := 0; i < st.NumFields(); i++ {
				field := st.Field(i)
				suffix := tn.Name() + "." + field.Name()
				canonical := path.Dir(rel) + "." + suffix
				if path.Dir(rel) == "." {
					canonical = "./" + suffix
				}
				id := ir.SymbolID(strings.TrimPrefix(owner, "module:"), canonical)
				visibility := "private"
				if field.Exported() {
					visibility = "public"
				}
				g.b.AddSymbol(ir.Symbol{ID: id, ModuleID: owner, Kind: "field", DisplaySignature: types.ObjectString(field, func(p *types.Package) string { return p.Path() }), Visibility: visibility})
				g.objects[field] = id
				g.symbolPackages[id] = p.PkgPath
				g.names[owner][suffix] = appendUnique(g.names[owner][suffix], id)
				g.names[owner][canonical] = appendUnique(g.names[owner][canonical], id)
			}
		}
	}
}

func (g *goAnalysis) resolveSeams(c config.Config) error {
	for name, m := range c.Modules {
		if m.Language != "go" {
			continue
		}
		owner := ir.ModuleID(name)
		for sn, seam := range m.Seams {
			sid := ir.SeamID(name, sn)
			for _, selector := range seam.Symbols {
				if err := g.resolveSelector(owner, sid, selector); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (g *goAnalysis) collectReferences(pkgs []*packages.Package, c config.Config) {
	seamPackages := map[string]map[string]bool{}
	for _, seam := range g.b.Partial.Seams {
		if seamPackages[seam.ModuleID] == nil {
			seamPackages[seam.ModuleID] = map[string]bool{}
		}
		for id := range g.b.Members[seam.ID] {
			seamPackages[seam.ModuleID][g.symbolPackages[id]] = true
		}
	}
	for _, p := range pkgs {
		for _, file := range p.Syntax {
			source := p.Fset.Position(file.Pos()).Filename
			owner := g.b.Owner(source)
			if owner == "" {
				continue
			}
			g.collectFileReferences(p, file, owner, c, seamPackages)
		}
	}
}

func (g *goAnalysis) collectImplementations(pkgs []*packages.Package) {
	// Structural satisfaction is a type fact, not proof of interface-based access.
	for _, p := range pkgs {
		for ident, obj := range p.TypesInfo.Defs {
			g.collectTypeImplementations(pkgs, p, ident, obj)
		}
	}
}

func (g *goAnalysis) collectDeclaration(p *packages.Package, ident *ast.Ident, obj types.Object) {
	if obj == nil || obj.Pkg() == nil {
		return
	}
	file := p.Fset.Position(ident.Pos()).Filename
	owner := g.b.Owner(file)
	if owner == "" {
		return
	}
	kind, suffix := g.declarationKind(p, obj, owner)
	if kind == "" {
		return
	}
	rel, _ := g.b.Relative(file)
	canonical := path.Dir(rel) + "." + suffix
	if path.Dir(rel) == "." {
		canonical = "./" + suffix
	}
	id := ir.SymbolID(strings.TrimPrefix(owner, "module:"), canonical)
	visibility := "private"
	if obj.Exported() {
		visibility = "public"
	}
	signature := types.ObjectString(obj, func(p *types.Package) string { return p.Path() })
	g.b.AddSymbol(ir.Symbol{ID: id, ModuleID: owner, Kind: kind, DisplaySignature: signature, Visibility: visibility})
	g.objects[obj] = id
	g.symbolPackages[id] = p.PkgPath
	if g.names[owner] == nil {
		g.names[owner] = map[string][]string{}
	}
	g.names[owner][obj.Name()] = append(g.names[owner][obj.Name()], id)
	g.names[owner][suffix] = appendUnique(g.names[owner][suffix], id)
	g.names[owner][canonical] = appendUnique(g.names[owner][canonical], id)
}

func (g *goAnalysis) declarationKind(p *packages.Package, obj types.Object, owner string) (string, string) {
	// Package declarations and named-type members; omit local variables.
	suffix := obj.Name()
	kind := ""
	switch x := obj.(type) {
	case *types.TypeName:
		if x.Parent() != p.Types.Scope() {
			return "", ""
		}
		kind = "type"
		if n, ok := types.Unalias(x.Type()).(*types.Named); ok {
			g.named[owner+":"+obj.Name()] = n
		}
	case *types.Func:
		kind = "function"
		sig := x.Type().(*types.Signature)
		if sig.Recv() != nil {
			recv := sig.Recv().Type()
			if ptr, ok := recv.(*types.Pointer); ok {
				recv = ptr.Elem()
			}
			n, ok := recv.(*types.Named)
			if !ok {
				return "", ""
			}
			suffix = n.Obj().Name() + "." + obj.Name()
			kind = "method"
		} else if x.Parent() != p.Types.Scope() {
			return "", ""
		}
	case *types.Var:
		if x.IsField() {
			return "", ""
		}
		if x.Parent() != p.Types.Scope() {
			return "", ""
		}
		kind = "variable"
	case *types.Const:
		if x.Parent() != p.Types.Scope() {
			return "", ""
		}
		kind = "constant"
	default:
		return "", ""
	}
	return kind, suffix
}

func (g *goAnalysis) resolveSelector(owner, sid, selector string) error {
	ids := g.names[owner][selector]
	if len(ids) != 1 {
		return fmt.Errorf("seam %s: Go selector %q resolved to %d symbols; use receiver-qualified names for methods", sid, selector, len(ids))
	}
	id := ids[0]
	if g.b.Symbols[id].Visibility != "public" {
		return fmt.Errorf("seam %s selects private symbol %s", sid, selector)
	}
	g.b.Members[sid][id] = true
	for selectorName, ids := range g.names[owner] {
		if strings.HasPrefix(selectorName, selector+".") {
			for _, memberID := range ids {
				if g.b.Symbols[memberID].Visibility == "public" {
					g.b.Members[sid][memberID] = true
				}
			}
		}
	}
	// An explicitly selected interface includes its method contract members.
	if n := g.named[owner+":"+selector]; n != nil {
		if iface, ok := n.Underlying().(*types.Interface); ok {
			iface.Complete()
			for i := 0; i < iface.NumMethods(); i++ {
				method := iface.Method(i)
				if mid := g.objects[method]; mid != "" {
					g.b.Members[sid][mid] = true
				}
			}
		}
	}
	return nil
}

func (g *goAnalysis) collectFileReferences(p *packages.Package, file *ast.File, owner string, c config.Config, seamPackages map[string]map[string]bool) {
	for _, imp := range file.Imports {
		g.collectImport(p, imp, owner, c, seamPackages)
	}
	contexts := g.referenceContexts(p, file)
	ast.Inspect(file, func(node ast.Node) bool {
		g.dynamicReference(p, node)
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		obj := p.TypesInfo.Uses[ident]
		id := g.objects[obj]
		if id != "" {
			g.b.Relation("uses", owner, id, contexts[ident], false)
		}
		return true
	})
}

func (g *goAnalysis) dynamicReference(p *packages.Package, node ast.Node) {
	if call, ok := node.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			obj := p.TypesInfo.Uses[sel.Sel]
			if obj != nil && obj.Pkg() != nil {
				pkg := obj.Pkg().Path()
				name := obj.Name()
				if (pkg == "plugin" && (name == "Open" || name == "Lookup")) || (pkg == "reflect" && (name == "Call" || name == "CallSlice" || name == "MethodByName" || name == "FieldByName")) {
					e := g.evidence(p, call.Pos(), call.End())
					e.Resolution = "dynamic"
					e.Confidence = "best-effort"
					g.b.Diagnostic("GO_DYNAMIC_REFERENCE", "Runtime reflection/plugin access is not statically proven", e)
				}
			}
		}
	}
}

func (g *goAnalysis) collectImport(p *packages.Package, imp *ast.ImportSpec, owner string, c config.Config, seamPackages map[string]map[string]bool) {
	importPath, _ := strconv.Unquote(imp.Path.Value)
	target := p.Imports[importPath]
	if target == nil {
		return
	}
	for _, f := range target.CompiledGoFiles {
		to := g.b.Owner(f)
		if rel, err := g.b.Relative(f); err == nil && to == "" && !strings.HasPrefix(rel, "vendor/") && !strings.HasPrefix(rel, ".build/") && !strings.HasPrefix(rel, "node_modules/") {
			e := g.evidence(p, imp.Pos(), imp.End())
			e.Resolution = "unresolved"
			e.Confidence = "best-effort"
			g.b.Diagnostic("GO_UNOWNED_PROJECT_REFERENCE", "Resolved local package has no declared architectural owner: "+importPath, e)
			break
		}
		if to != "" && c.Modules[strings.TrimPrefix(to, "module:")].Language != "go" {
			e := g.evidence(p, imp.Pos(), imp.End())
			e.Resolution = "unresolved"
			e.Confidence = "best-effort"
			g.b.Diagnostic("GO_TARGET_ANALYSIS_MISMATCH", "Imported Go source belongs to a module configured for a different adapter", e)
			break
		}
		if to != "" && to != owner {
			bypass := seamPackages[to] != nil && !seamPackages[to][target.PkgPath]
			g.b.Relation("imports", owner, to, g.evidence(p, imp.Pos(), imp.End()), bypass)
			break
		}
	}
}

func (g *goAnalysis) collectTypeImplementations(pkgs []*packages.Package, p *packages.Package, ident *ast.Ident, obj types.Object) {
	tn, ok := obj.(*types.TypeName)
	if !ok {
		return
	}
	n, ok := types.Unalias(tn.Type()).(*types.Named)
	if !ok || g.objects[obj] == "" {
		return
	}
	if _, ok := n.Underlying().(*types.Interface); ok {
		return
	}
	for _, other := range pkgs {
		for _, o := range other.TypesInfo.Defs {
			it, ok := o.(*types.TypeName)
			if !ok || g.objects[it] == "" || g.objects[it] == g.objects[obj] {
				continue
			}
			iface, ok := it.Type().Underlying().(*types.Interface)
			if !ok || iface.NumMethods() == 0 {
				continue
			}
			if types.Implements(n, iface) || types.Implements(types.NewPointer(n), iface) {
				g.b.Relation("implements", g.objects[obj], g.objects[it], g.evidence(p, ident.Pos(), ident.End()), false)
			}
		}
	}
}
