package ui

import (
	"net/url"
	"slices"
	"strings"

	"seamgraph/internal/application"
	"seamgraph/internal/ir"
)

type CodeFilter struct {
	Query, Kind, Visibility, Status, View, CommitType, ReviewIssue string
	ChangesOnly, Tools                                             bool
}
type codeSymbolNode struct {
	Object   application.CodeObject
	Children []*codeSymbolNode
}
type codeTreeView struct {
	Files   []*reviewNode
	Symbols map[string][]*codeSymbolNode
}

func codeLink(browser application.CodeBrowser, filter CodeFilter, file, object string) string {
	revision := codeRevision(browser)
	base := codeRef(browser.Base)
	if browser.ReviewHeadRef != "" {
		revision = browser.ReviewHeadRef
		base = browser.ReviewBaseRef
	}
	values := url.Values{"revision": {revision}, "base": {base}, "file": {file}, "object": {object}, "q": {filter.Query}, "kind": {filter.Kind}, "visibility": {filter.Visibility}, "status": {filter.Status}, "view": {filter.View}, "type": {filter.CommitType}}
	if filter.ChangesOnly {
		values.Set("changes", "1")
	}
	if filter.Tools {
		values.Set("tools", "1")
	}
	return "/explore?" + values.Encode()
}

func codeMatch(object application.CodeObject, filter CodeFilter) bool {
	if filter.ChangesOnly && object.Status == "unchanged" {
		return false
	}
	if filter.Kind != "" && filter.Kind != object.Symbol.Kind {
		return false
	}
	if filter.Visibility != "" && filter.Visibility != object.Symbol.Visibility {
		return false
	}
	if filter.Status != "" && filter.Status != object.Status {
		return false
	}
	text := object.Symbol.ID + " " + object.Symbol.DisplaySignature
	if object.Symbol.Definition != nil {
		text += " " + object.Symbol.Definition.Name + " " + object.Symbol.Definition.Location.File
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(filter.Query))
}
func codeTree(browser application.CodeBrowser, filter CodeFilter) codeTreeView {
	tree := codeTreeView{Symbols: map[string][]*codeSymbolNode{}}
	nodes := map[string]*codeSymbolNode{}
	for _, object := range browser.Objects {
		nodes[object.Symbol.ID] = &codeSymbolNode{Object: object}
	}
	allowed := map[string]bool{}
	for _, object := range browser.Objects {
		if !codeMatch(object, filter) {
			continue
		}
		for node := nodes[object.Symbol.ID]; node != nil && !allowed[node.Object.Symbol.ID]; {
			allowed[node.Object.Symbol.ID] = true
			definition := node.Object.Symbol.Definition
			if definition == nil {
				break
			}
			node = nodes[definition.ParentID]
		}
	}
	paths := attachCodeNodes(nodes, allowed, &tree)
	files := []application.ReviewFile{}
	for _, file := range browser.Files {
		if filter.ChangesOnly && file.Status == "unchanged" {
			continue
		}
		direct := filter.Kind == "" && filter.Visibility == "" && (filter.Status == "" || filter.Status == file.Status) && strings.Contains(strings.ToLower(file.Path), strings.ToLower(filter.Query))
		if direct || paths[file.Path] {
			files = append(files, file)
		}
	}
	tree.Files = reviewTree(files, "")
	return tree
}
func attachCodeNodes(nodes map[string]*codeSymbolNode, allowed map[string]bool, tree *codeTreeView) map[string]bool {
	paths := map[string]bool{}
	for id, node := range nodes {
		if !allowed[id] || node.Object.Symbol.Definition == nil {
			continue
		}
		definition := node.Object.Symbol.Definition
		paths[definition.Location.File] = true
		parent := nodes[definition.ParentID]
		if parent != nil && allowed[definition.ParentID] {
			parent.Children = append(parent.Children, node)
		}
		if parent == nil || parent.Object.Symbol.Definition == nil || parent.Object.Symbol.Definition.Location.File != definition.Location.File {
			tree.Symbols[definition.Location.File] = append(tree.Symbols[definition.Location.File], node)
		}
	}
	for _, nodes := range tree.Symbols {
		sortCodeNodes(nodes)
	}
	return paths
}
func sortCodeNodes(nodes []*codeSymbolNode) {
	slices.SortFunc(nodes, func(a, b *codeSymbolNode) int { return strings.Compare(a.Object.Symbol.ID, b.Object.Symbol.ID) })
	for _, node := range nodes {
		sortCodeNodes(node.Children)
	}
}
func selectedCode(browser application.CodeBrowser) *application.CodeObject {
	for _, object := range browser.Objects {
		if object.Symbol.ID == browser.Object {
			return &object
		}
	}
	return nil
}
func codeKinds(browser application.CodeBrowser) []string {
	kinds := []string{}
	for _, o := range browser.Objects {
		if !slices.Contains(kinds, o.Symbol.Kind) {
			kinds = append(kinds, o.Symbol.Kind)
		}
	}
	slices.Sort(kinds)
	return kinds
}
func codeReferences(browser application.CodeBrowser, view string) []ir.Relation {
	result := []ir.Relation{}
	for _, r := range browser.Relations {
		incoming := r.ToID == browser.Object
		outgoing := r.FromID == browser.Object
		if !incoming && !outgoing {
			continue
		}
		switch view {
		case "callers":
			if !incoming || len(r.Evidence) == 0 || r.Evidence[0].Kind != "call" {
				continue
			}
		case "outgoing":
			if !outgoing || !ir.IsConsumerRelation(r) {
				continue
			}
		case "types":
			if r.Kind != "implements" {
				continue
			}
		default:
			if !incoming || !ir.IsConsumerRelation(r) {
				continue
			}
		}
		result = append(result, r)
	}
	slices.SortFunc(result, func(a, b ir.Relation) int { return strings.Compare(a.ID, b.ID) })
	return result
}

func codeRevision(browser application.CodeBrowser) string { return codeRef(browser.Revision) }
func codeRef(identity string) string {
	if identity == "Working directory" {
		return ""
	}
	if strings.HasPrefix(identity, "WORKTREE:") {
		return "WORKTREE"
	}
	return identity
}
func codeName(object application.CodeObject) string {
	if object.Symbol.Definition != nil {
		return object.Symbol.Definition.Name
	}
	return object.Symbol.ID
}
func codeAncestor(nodes []*codeSymbolNode, selected, id string) bool {
	for _, node := range nodes {
		if node.Object.Symbol.ID == id {
			return codeContains(node, selected)
		}
	}
	return false
}
func codeContains(node *codeSymbolNode, selected string) bool {
	if node.Object.Symbol.ID == selected {
		return true
	}
	for _, child := range node.Children {
		if codeContains(child, selected) {
			return true
		}
	}
	return false
}

func comparisonLink(browser application.CodeBrowser, filter CodeFilter, compare bool) string {
	if compare {
		browser.ReviewBaseRef = "HEAD"
		browser.ReviewHeadRef = codeRevision(browser)
		if browser.ReviewHeadRef == "" {
			browser.ReviewHeadRef = "WORKTREE"
		}
		filter.ChangesOnly = true
	} else {
		for _, file := range browser.Files {
			if file.Path == browser.Selected && file.Status == "removed" {
				browser.Revision = browser.Base
			}
		}
		if object := selectedCode(browser); object != nil && object.Status == "removed" {
			browser.Revision = browser.Base
		}
		browser.Base = ""
		browser.ReviewBaseRef = ""
		browser.ReviewHeadRef = ""
		filter.ChangesOnly = false
	}
	return codeLink(browser, filter, browser.Selected, browser.Object)
}
