package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"seamgraph/internal/application"
	"seamgraph/internal/ui"
)

func (a *App) review(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	base, head := query.Get("base"), query.Get("head")
	if base == "" {
		base = "HEAD"
	}
	if head == "" {
		head = "HEAD"
	}
	result, err := a.backend.Review(r.Context(), base, head, query.Get("file"), query.Get("object"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result.PathFilter = query.Get("q")
	result.TypeFilter = query.Get("type")
	switch query.Get("export") {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="seamgraph-review.json"`)
		_ = json.NewEncoder(w).Encode(result)
	case "markdown":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="seamgraph-review.md"`)
		// #nosec G705 -- This text/markdown attachment is nosniff, not HTML; embedded source and notes are JSON-escaped.
		_, _ = fmt.Fprintf(w, "# Local code review\n\nComparison: %s to %s (two trees).\n\nActivity: %s\n\nFull source includes only the selected file; saved findings also include their original bounded source excerpts.\n\nThe complete comparison, source references, findings and original note anchors follow.\n\n```json\n%s\n```\n", result.Comparison.Base, result.Comparison.Head, result.Activity.Policy, application.ReviewJSON(result))
	case "html":
		w.Header().Set("Content-Disposition", `attachment; filename="seamgraph-review.html"`)
		render(w, r, ui.ReviewSnapshot(result))
	default:
		render(w, r, ui.ReviewPage(a.token, result, query.Get("q"), query.Get("type")))
	}
}

func (a *App) recordReview(w http.ResponseWriter, r *http.Request) {
	if !a.validateWrite(w, r) {
		return
	}
	entry := application.ReviewEntry{BaseRef: r.Form.Get("baseRef"), HeadRef: r.Form.Get("headRef"), Base: r.Form.Get("base"), Head: r.Form.Get("head"), File: r.Form.Get("file"), Note: r.Form.Get("note"), Viewed: r.Form.Get("viewed") == "on", Resolved: r.Form.Get("resolved") == "on"}
	if err := a.backend.RecordReview(r.Context(), entry); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	query := url.Values{"base": {entry.BaseRef}, "head": {entry.HeadRef}, "file": {entry.File}}
	http.Redirect(w, r, "/review?"+query.Encode(), http.StatusSeeOther)
}
