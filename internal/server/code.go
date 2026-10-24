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
		q.Set("head", q.Get("revision"))
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
	var review *application.ReviewWorkspace
	var reviewIssue string
	if base != "" {
		workspace, reviewErr := a.backend.Review(r.Context(), base, revision, q.Get("file"), q.Get("object"))
		if reviewErr == nil {
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
	if q.Get("format") == "json" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(browser)
		return
	}
	filter := ui.CodeFilter{Query: q.Get("q"), Kind: q.Get("kind"), Visibility: q.Get("visibility"), Status: q.Get("status"), View: q.Get("view"), CommitType: q.Get("type"), ChangesOnly: q.Get("changes") == "1", Tools: q.Get("tools") == "1", ReviewIssue: reviewIssue}
	render(w, r, ui.CodePage(a.token, browser, filter, review))
}

func comparisonRefs(r *http.Request) (string, string) {
	q := r.URL.Query()
	switch q.Get("scope") {
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
