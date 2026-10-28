package server

import (
	"encoding/json"
	"net/http"
	"seamgraph/internal/application"
	"seamgraph/internal/ui"
)

func (a *App) code(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("export") == "comments" {
		a.commentExport(w, r)
		return
	}
	if q.Get("export") != "" {
		revision, base := comparisonRefs(r)
		q.Set("head", revision)
		q.Set("base", base)
		q.Set("scope", "revisions")
		r.URL.RawQuery = q.Encode()
		a.review(w, r)
		return
	}
	revision, base := comparisonRefs(r)
	if base != "" && revision == "" {
		revision = "WORKTREE"
	}
	browser, err := a.backend.Code(r.Context(), revision, base, q.Get("file"), q.Get("object"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	browser.Options = reviewOptions(q)
	var review *application.ReviewWorkspace
	var reviewIssue string
	if base != "" {
		workspace, reviewErr := a.backend.Review(r.Context(), base, revision, q.Get("file"), q.Get("object"))
		if reviewErr == nil {
			workspace.Options = reviewOptions(q)
			workspace.Code.Options = workspace.Options
			workspace.PathFilter = q.Get("q")
			workspace.TypeFilter = q.Get("type")
			workspace.Summary = application.SummarizeReview(workspace)
			review = &workspace
			review.PathFilter = q.Get("q")
			review.TypeFilter = q.Get("type")
			browser = workspace.Code
			browser.Marks = workspace.Marks
		} else {
			reviewIssue = reviewErr.Error()
		}
		browser.ReviewBaseRef = base
		browser.ReviewHeadRef = revision
	}
	if q.Get("evidence") == "before" && browser.Base != "" {
		browser.Relations = browser.BeforeRelations
		browser.RelationshipRevision = browser.Base
	}
	if q.Get("evidence") == "after" && browser.Base != "" {
		browser.Relations = browser.AfterRelations
		browser.RelationshipRevision = browser.Revision
	}
	if q.Get("format") == "json" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(browser)
		return
	}
	filter := ui.CodeFilter{Branches: a.backend.Branches(r.Context()), Tab: q.Get("tab"), Query: q.Get("q"), Kind: q.Get("kind"), Visibility: q.Get("visibility"), Status: q.Get("status"), View: q.Get("view"), CommitType: q.Get("type"), ChangesOnly: q.Get("changes") == "1", Tools: q.Get("tools") == "1", ReviewIssue: reviewIssue}
	render(w, r, ui.CodePage(a.token, browser, filter, review))
}

func comparisonRefs(r *http.Request) (string, string) {
	q := r.URL.Query()
	switch q.Get("scope") {
	case "branches":
		return q.Get("head"), "merge-base:" + q.Get("base")
	case "target-tip":
		return q.Get("head"), "target-tip:" + q.Get("base")
	case "working":
		return "WORKTREE", "HEAD"
	case "revisions":
		return q.Get("head"), q.Get("base")
	default:
		revision := q.Get("revision")
		if head := q.Get("head"); head != "" {
			revision = head
		}
		if revision == "WORKTREE" && q.Get("base") == "" {
			revision = ""
		}
		return revision, q.Get("base")
	}
}

func reviewOptions(q map[string][]string) map[string]string {
	result := map[string]string{}
	for _, key := range []string{"layout", "metric", "sort", "exclude", "group", "commitScope", "historyFile", "historyGroup", "evidence"} {
		if values := q[key]; len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}
