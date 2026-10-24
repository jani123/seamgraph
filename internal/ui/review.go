package ui

import (
	"net/url"
	"slices"
	"strings"

	"seamgraph/internal/application"
	"seamgraph/internal/ir"
)

type reviewNode struct {
	Name, Path, Status string
	Children           []*reviewNode
}

func reviewTree(files []application.ReviewFile, query string) []*reviewNode {
	root := &reviewNode{}
	nodes := map[string]*reviewNode{"": root}
	for _, file := range files {
		if !strings.Contains(strings.ToLower(file.Path), strings.ToLower(query)) {
			continue
		}
		parent := ""
		for _, name := range strings.Split(file.Path, "/") {
			path := name
			if parent != "" {
				path = parent + "/" + name
			}
			if nodes[path] == nil {
				nodes[path] = &reviewNode{Name: name, Path: path}
				nodes[parent].Children = append(nodes[parent].Children, nodes[path])
			}
			parent = path
		}
		nodes[file.Path].Status = file.Status
	}
	return root.Children
}
func reviewLink(r application.ReviewWorkspace, file, object, export, kind string) string {
	return "/review?" + url.Values{"base": {r.BaseRef}, "head": {r.HeadRef}, "file": {file}, "object": {object}, "export": {export}, "type": {kind}, "q": {r.PathFilter}}.Encode()
}
func reviewMark(r application.ReviewWorkspace, file string) application.ReviewMark {
	for _, mark := range r.Marks {
		if mark.Entry.File == file {
			return mark
		}
	}
	return application.ReviewMark{}
}
func reviewCommits(r application.ReviewWorkspace, kind string) []ir.ReviewCommit {
	result := []ir.ReviewCommit{}
	for _, c := range r.Activity.Commits {
		if kind != "" && c.Type != kind {
			continue
		}
		if r.Selected != "" {
			if _, ok := c.Files[r.Selected]; !ok {
				continue
			}
		}
		result = append(result, c)
	}
	return result
}
func reviewTypes(r application.ReviewWorkspace) []string {
	types := []string{}
	for _, c := range r.Activity.Commits {
		if !slices.Contains(types, c.Type) {
			types = append(types, c.Type)
		}
	}
	slices.Sort(types)
	return types
}
func reviewTypeCount(r application.ReviewWorkspace, kind string) int {
	n := 0
	for _, c := range r.Activity.Commits {
		if c.Type == kind {
			n++
		}
	}
	return n
}
func reviewSubjects(r application.ReviewWorkspace) []string {
	ids := []string{}
	for _, c := range r.Comparison.Diff.Changes {
		if c.Kind == "symbol" {
			ids = append(ids, c.EntityID)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}
func hotspotVisible(r application.ReviewWorkspace, h application.Hotspot, kind, query string) bool {
	if !strings.Contains(strings.ToLower(h.Path), strings.ToLower(query)) {
		return false
	}
	if kind == "" {
		return true
	}
	for _, commit := range r.Activity.Commits {
		if commit.Type == kind {
			if _, ok := commit.Files[h.Path]; ok {
				return true
			}
		}
	}
	return false
}
