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
	"seamgraph/internal/analyzer"
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
	for name, m := range c.Modules {
		if m.Language == "go" {
			_ = name
			for _, root := range m.Paths {
				patterns = append(patterns, "./"+root+"/...")
			}
		}
	}
	buildFlags := []string{"-mod=readonly"}
	if _, err := os.Stat(filepath.Join(b.Root, "vendor", "modules.txt")); err == nil {
		buildFlags = []string{"-mod=vendor"}
	}
	pkgs, err := packages.Load(&packages.Config{Context: ctx, Dir: b.Root, BuildFlags: buildFlags, Env: append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local"), Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule}, patterns...)
	if err != nil {
		return b.Partial, err
	}
	if len(pkgs) == 0 {
		return b.Partial, fmt.Errorf("no Go packages found")
	}
	var loadErr error
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			if loadErr == nil {
				loadErr = fmt.Errorf("Go analysis: %s", e.Error())
			}
		}
	})
	if loadErr != nil {
		return b.Partial, loadErr
	}
	objects := map[types.Object]string{}
	symbolPackages := map[string]string{}
	names := map[string]map[string][]string{}
	named := map[string]*types.Named{}
	evidence := func(p *packages.Package, pos, end token.Pos) ir.Evidence {
		a, z := p.Fset.Position(pos), p.Fset.Position(end)
		file, _ := b.Relative(a.Filename)
		return ir.Evidence{File: file, StartLine: a.Line, StartColumn: a.Column, EndLine: z.Line, EndColumn: z.Column, Confidence: "exact", Resolution: "resolved"}
	}
	for _, p := range pkgs {
		for ident, obj := range p.TypesInfo.Defs {
			if obj == nil || obj.Pkg() == nil {
				continue
			}
			file := p.Fset.Position(ident.Pos()).Filename
			owner := b.Owner(file)
			if owner == "" {
				continue
			}
			// Package declarations and named-type members; omit local variables.
			suffix := obj.Name()
			kind := ""
			switch x := obj.(type) {
			case *types.TypeName:
				if x.Parent() != p.Types.Scope() {
					continue
				}
				kind = "type"
				if n, ok := types.Unalias(x.Type()).(*types.Named); ok {
					named[owner+":"+obj.Name()] = n
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
						continue
					}
					suffix = n.Obj().Name() + "." + obj.Name()
					kind = "method"
				} else if x.Parent() != p.Types.Scope() {
					continue
				}
			case *types.Var:
				if x.IsField() {
					continue
				}
				if x.Parent() != p.Types.Scope() {
					continue
				}
				kind = "variable"
			case *types.Const:
				if x.Parent() != p.Types.Scope() {
					continue
				}
				kind = "constant"
			default:
				continue
			}
			rel, _ := b.Relative(file)
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
			b.AddSymbol(ir.Symbol{ID: id, ModuleID: owner, Kind: kind, DisplaySignature: signature, Visibility: visibility})
			objects[obj] = id
			symbolPackages[id] = p.PkgPath
			if names[owner] == nil {
				names[owner] = map[string][]string{}
			}
			names[owner][obj.Name()] = append(names[owner][obj.Name()], id)
			names[owner][suffix] = appendUnique(names[owner][suffix], id)
			names[owner][canonical] = appendUnique(names[owner][canonical], id)
		}
	}
	// Struct fields are real cross-module references too, not just declarations.
	for _, p := range pkgs {
		for ident, obj := range p.TypesInfo.Defs {
			tn, ok := obj.(*types.TypeName)
			if !ok || objects[tn] == "" {
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
			owner := b.Owner(file)
			rel, _ := b.Relative(file)
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
				b.AddSymbol(ir.Symbol{ID: id, ModuleID: owner, Kind: "field", DisplaySignature: types.ObjectString(field, func(p *types.Package) string { return p.Path() }), Visibility: visibility})
				objects[field] = id
				symbolPackages[id] = p.PkgPath
				names[owner][suffix] = appendUnique(names[owner][suffix], id)
				names[owner][canonical] = appendUnique(names[owner][canonical], id)
			}
		}
	}
	for name, m := range c.Modules {
		if m.Language != "go" {
			continue
		}
		owner := ir.ModuleID(name)
		for sn, seam := range m.Seams {
			sid := ir.SeamID(name, sn)
			for _, selector := range seam.Symbols {
				ids := names[owner][selector]
				if len(ids) != 1 {
					return b.Partial, fmt.Errorf("seam %s: Go selector %q resolved to %d symbols; use receiver-qualified names for methods", sid, selector, len(ids))
				}
				id := ids[0]
				if b.Symbols[id].Visibility != "public" {
					return b.Partial, fmt.Errorf("seam %s selects private symbol %s", sid, selector)
				}
				b.Members[sid][id] = true
				for selectorName, ids := range names[owner] {
					if strings.HasPrefix(selectorName, selector+".") {
						for _, memberID := range ids {
							if b.Symbols[memberID].Visibility == "public" {
								b.Members[sid][memberID] = true
							}
						}
					}
				}
				// An explicitly selected interface includes its method contract members.
				if n := named[owner+":"+selector]; n != nil {
					if iface, ok := n.Underlying().(*types.Interface); ok {
						iface.Complete()
						for i := 0; i < iface.NumMethods(); i++ {
							method := iface.Method(i)
							if mid := objects[method]; mid != "" {
								b.Members[sid][mid] = true
							}
						}
					}
				}
			}
		}
	}
	seamPackages := map[string]map[string]bool{}
	for _, seam := range b.Partial.Seams {
		if seamPackages[seam.ModuleID] == nil {
			seamPackages[seam.ModuleID] = map[string]bool{}
		}
		for id := range b.Members[seam.ID] {
			seamPackages[seam.ModuleID][symbolPackages[id]] = true
		}
	}
	for _, p := range pkgs {
		for _, file := range p.Syntax {
			source := p.Fset.Position(file.Pos()).Filename
			owner := b.Owner(source)
			if owner == "" {
				continue
			}
			for _, imp := range file.Imports {
				importPath, _ := strconv.Unquote(imp.Path.Value)
				target := p.Imports[importPath]
				if target == nil {
					continue
				}
				for _, f := range target.CompiledGoFiles {
					to := b.Owner(f)
					if rel, err := b.Relative(f); err == nil && to == "" && !strings.HasPrefix(rel, "vendor/") && !strings.HasPrefix(rel, ".build/") && !strings.HasPrefix(rel, "node_modules/") {
						e := evidence(p, imp.Pos(), imp.End())
						e.Resolution = "unresolved"
						e.Confidence = "best-effort"
						b.Diagnostic("GO_UNOWNED_PROJECT_REFERENCE", "Resolved local package has no declared architectural owner: "+importPath, e)
						break
					}
					if to != "" && c.Modules[strings.TrimPrefix(to, "module:")].Language != "go" {
						e := evidence(p, imp.Pos(), imp.End())
						e.Resolution = "unresolved"
						e.Confidence = "best-effort"
						b.Diagnostic("GO_TARGET_ANALYSIS_MISMATCH", "Imported Go source belongs to a module configured for a different adapter", e)
						break
					}
					if to != "" && to != owner {
						bypass := seamPackages[to] != nil && !seamPackages[to][target.PkgPath]
						b.Relation("imports", owner, to, evidence(p, imp.Pos(), imp.End()), bypass)
						break
					}
				}
			}
			ast.Inspect(file, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						obj := p.TypesInfo.Uses[sel.Sel]
						if obj != nil && obj.Pkg() != nil {
							pkg := obj.Pkg().Path()
							name := obj.Name()
							if (pkg == "plugin" && (name == "Open" || name == "Lookup")) || (pkg == "reflect" && (name == "Call" || name == "CallSlice" || name == "MethodByName" || name == "FieldByName")) {
								e := evidence(p, call.Pos(), call.End())
								e.Resolution = "dynamic"
								e.Confidence = "best-effort"
								b.Diagnostic("GO_DYNAMIC_REFERENCE", "Runtime reflection/plugin access is not statically proven", e)
							}
						}
					}
				}
				ident, ok := node.(*ast.Ident)
				if !ok {
					return true
				}
				obj := p.TypesInfo.Uses[ident]
				id := objects[obj]
				if id != "" && b.Symbols[id].ModuleID != owner {
					b.Relation("uses", owner, id, evidence(p, ident.Pos(), ident.End()), false)
				}
				return true
			})
		}
	}
	// Structural satisfaction is a type fact, not proof of interface-based access.
	for _, p := range pkgs {
		for ident, obj := range p.TypesInfo.Defs {
			tn, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}
			n, ok := types.Unalias(tn.Type()).(*types.Named)
			if !ok || objects[obj] == "" {
				continue
			}
			if _, ok := n.Underlying().(*types.Interface); ok {
				continue
			}
			for _, other := range pkgs {
				for _, o := range other.TypesInfo.Defs {
					it, ok := o.(*types.TypeName)
					if !ok || objects[it] == "" || objects[it] == objects[obj] {
						continue
					}
					iface, ok := it.Type().Underlying().(*types.Interface)
					if !ok || iface.NumMethods() == 0 {
						continue
					}
					if types.Implements(n, iface) || types.Implements(types.NewPointer(n), iface) {
						b.Relation("implements", objects[obj], objects[it], evidence(p, ident.Pos(), ident.End()), false)
					}
				}
			}
		}
	}
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
