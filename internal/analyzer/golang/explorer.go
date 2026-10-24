package golang

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/printer"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
	"seamgraph/internal/ir"
)

// Declaration metadata is collected after symbol identities, avoiding map-order
// dependencies between a type and its methods or fields.
func (g *goAnalysis) collectDefinitions(pkgs []*packages.Package) {
	for _, p := range pkgs {
		for _, file := range p.Syntax {
			g.fileDefinitions(p, file)
		}
	}
}
func (g *goAnalysis) fileDefinitions(p *packages.Package, file *ast.File) {
	parents := map[ast.Node]ast.Node{}
	stack := []ast.Node{}
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	for ident, obj := range p.TypesInfo.Defs {
		id := g.objects[obj]
		if id == "" || ident.Pos() < file.Pos() || ident.Pos() > file.End() {
			continue
		}
		symbol := g.b.Symbols[id]
		definition := &ir.Definition{Name: ident.Name, Location: g.evidence(p, ident.Pos(), ident.End())}
		definition.ParentID = g.declarationParent(obj)
		for node := parents[ident]; node != nil; node = parents[node] {
			if definition.Digest == "" && declarationNode(node) {
				var text bytes.Buffer
				if err := printer.Fprint(&text, p.Fset, node); err == nil {
					definition.Digest = fmt.Sprintf("%x", sha256.Sum256(text.Bytes()))
				}
			}
			if doc := goDocumentation(node); doc != "" {
				definition.Documentation = doc
				break
			}
			if _, ok := node.(*ast.Field); ok {
				break
			}
			if _, ok := node.(*ast.FuncDecl); ok {
				break
			}
			if _, ok := node.(*ast.File); ok {
				break
			}
		}
		symbol.Definition = definition
		g.b.Symbols[id] = symbol
	}
}
func goDocumentation(node ast.Node) string {
	var group *ast.CommentGroup
	switch n := node.(type) {
	case *ast.FuncDecl:
		group = n.Doc
	case *ast.GenDecl:
		group = n.Doc
	case *ast.TypeSpec:
		group = n.Doc
	case *ast.ValueSpec:
		group = n.Doc
	case *ast.Field:
		group = n.Doc
	}
	if group != nil {
		return strings.TrimSpace(group.Text())
	}
	return ""
}
func (g *goAnalysis) declarationParent(obj types.Object) string {
	if fn, ok := obj.(*types.Func); ok {
		sig, ok := fn.Type().(*types.Signature)
		if ok && sig.Recv() != nil {
			typ := sig.Recv().Type()
			if ptr, ok := typ.(*types.Pointer); ok {
				typ = ptr.Elem()
			}
			if named, ok := typ.(*types.Named); ok {
				return g.objects[named.Obj()]
			}
		}
	}
	if field, ok := obj.(*types.Var); ok && field.IsField() {
		for object, id := range g.objects {
			tn, ok := object.(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			structure, ok := tn.Type().Underlying().(*types.Struct)
			if !ok {
				continue
			}
			for i := 0; i < structure.NumFields(); i++ {
				if structure.Field(i) == field && strings.HasPrefix(g.objects[field], id+".") {
					return id
				}
			}
		}
	}
	return ""
}
func declarationNode(node ast.Node) bool {
	switch node.(type) {
	case *ast.FuncDecl, *ast.TypeSpec, *ast.ValueSpec, *ast.Field:
		return true
	}
	return false
}
func (g *goAnalysis) referenceContexts(p *packages.Package, file *ast.File) map[*ast.Ident]ir.Evidence {
	contexts := map[*ast.Ident]ir.Evidence{}
	stack := []ast.Node{}
	sources := []string{}
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			sources = sources[:len(sources)-1]
			return true
		}
		source := ""
		if len(sources) > 0 {
			source = sources[len(sources)-1]
		}
		if id := g.declarationSource(p, node); id != "" {
			source = id
		}
		if ident, ok := node.(*ast.Ident); ok {
			location := g.evidence(p, ident.Pos(), ident.End())
			location.SourceID = source
			location.Kind = "reference"
			if directCall(ident, stack) {
				location.Kind = "call"
			}
			contexts[ident] = location
		}
		stack = append(stack, node)
		sources = append(sources, source)
		return true
	})
	return contexts
}
func (g *goAnalysis) declarationSource(p *packages.Package, node ast.Node) string {
	switch n := node.(type) {
	case *ast.FuncDecl:
		return g.objects[p.TypesInfo.Defs[n.Name]]
	case *ast.TypeSpec:
		return g.objects[p.TypesInfo.Defs[n.Name]]
	case *ast.ValueSpec:
		if len(n.Names) == 1 {
			return g.objects[p.TypesInfo.Defs[n.Names[0]]]
		}
	}
	return ""
}
func directCall(ident *ast.Ident, stack []ast.Node) bool {
	if len(stack) == 0 {
		return false
	}
	parent := stack[len(stack)-1]
	if call, ok := parent.(*ast.CallExpr); ok {
		return call.Fun == ident
	}
	if selector, ok := parent.(*ast.SelectorExpr); ok && selector.Sel == ident && len(stack) > 1 {
		if call, ok := stack[len(stack)-2].(*ast.CallExpr); ok {
			return call.Fun == selector
		}
	}
	return false
}

// Derived struct types reuse the original field declarations. Only literal
// struct declarations introduce new field identities.
func declaredStructs(p *packages.Package) map[*types.TypeName]bool {
	result := map[*types.TypeName]bool{}
	for _, file := range p.Syntax {
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := spec.Type.(*ast.StructType); !ok || spec.Assign.IsValid() {
				return false
			}
			if name, ok := p.TypesInfo.Defs[spec.Name].(*types.TypeName); ok {
				result[name] = true
			}
			return false
		})
	}
	return result
}
