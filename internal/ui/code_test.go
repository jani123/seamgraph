package ui

import (
	"net/url"
	"seamgraph/internal/application"
	"seamgraph/internal/ir"
	"testing"
)

func TestCodeNavigationKeepsLiveRefsUsable(t *testing.T) {
	browser := application.CodeBrowser{Revision: "WORKTREE:content", Base: "WORKTREE:base"}
	link, err := url.Parse(codeLink(browser, CodeFilter{}, "code.go", ""))
	if err != nil {
		t.Fatal(err)
	}
	if link.Query().Get("revision") != "WORKTREE" || link.Query().Get("base") != "WORKTREE" {
		t.Fatal(link)
	}
	browser.Revision = "Working directory"
	if codeRevision(browser) != "" {
		t.Fatal("live directory became a git ref")
	}
	browser.ReviewBaseRef = "target"
	browser.ReviewHeadRef = "review"
	link, err = url.Parse(codeLink(browser, CodeFilter{Kind: "method"}, "", "symbol:orders:Service.Place"))
	if err != nil {
		t.Fatal(err)
	}
	if link.Path != "/review" || link.Query().Get("head") != "review" || link.Query().Get("symbolKind") != "method" {
		t.Fatal(link)
	}
}

func TestCodeConsumersExcludeInferredTypeConformance(t *testing.T) {
	browser := application.CodeBrowser{Object: "reader", Relations: []ir.Relation{
		{ID: "observed", Kind: "uses", FromID: "caller", ToID: "reader", Evidence: []ir.Evidence{{Kind: "call"}}},
		{ID: "inferred", Kind: "implements", FromID: "store", ToID: "reader"},
	}}
	for _, view := range []string{"consumers", "callers"} {
		relations := codeReferences(browser, view)
		if len(relations) != 1 || relations[0].ID != "observed" {
			t.Fatal(view, relations)
		}
	}
	relations := codeReferences(browser, "types")
	if len(relations) != 1 || relations[0].ID != "inferred" {
		t.Fatal(relations)
	}
}
