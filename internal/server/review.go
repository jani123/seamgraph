package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"seamgraph/internal/application"
	"seamgraph/internal/ui"
)

func (a *App) review(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if query.Get("export") == "" {
		if query.Get("base") == "" {
			query.Set("base", "HEAD")
		}
		if query.Get("head") == "" {
			query.Set("head", "HEAD")
		}
		query.Set("revision", query.Get("head"))
		query.Del("head")
		query.Set("tools", "1")
		if query.Get("symbolKind") != "" {
			query.Set("kind", query.Get("symbolKind"))
			query.Del("symbolKind")
		}
		// #nosec G710 -- The destination is the fixed local /explore path. url.Values.Encode escapes all untrusted query keys/values, so they cannot change the path or origin.
		http.Redirect(w, r, "/explore?"+query.Encode(), http.StatusSeeOther)
		return
	}
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
		http.Error(w, "unknown review export", http.StatusBadRequest)
	}
}

func (a *App) recordReview(w http.ResponseWriter, r *http.Request) {
	if !a.validateWrite(w, r) {
		return
	}
	entry := application.ReviewEntry{BaseRef: r.Form.Get("baseRef"), HeadRef: r.Form.Get("headRef"), Base: r.Form.Get("base"), Head: r.Form.Get("head"), File: r.Form.Get("file"), Note: r.Form.Get("note"), Viewed: r.Form.Get("viewed") == "on", Resolved: r.Form.Get("resolved") == "on"}
	entry.Comment = r.Form.Get("comment") == "1"
	entry.Side = r.Form.Get("side")
	if value := r.Form.Get("line"); value != "" {
		line, err := strconv.Atoi(value)
		if err != nil {
			http.Error(w, "invalid comment line", http.StatusBadRequest)
			return
		}
		entry.Line = line
	}
	if err := a.backend.RecordReview(r.Context(), entry); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	query := url.Values{"tools": {"1"}, "base": {entry.BaseRef}, "revision": {entry.HeadRef}, "file": {entry.File}}
	http.Redirect(w, r, "/explore?"+query.Encode(), http.StatusSeeOther)
}

func (a *App) commentExport(w http.ResponseWriter, r *http.Request) {
	head, base := comparisonRefs(r)
	if base == "" {
		http.Error(w, "select a comparison before exporting comments", http.StatusBadRequest)
		return
	}
	code, err := a.backend.Code(r.Context(), head, base, "", "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="review-comments.md"`)
	// #nosec G705 -- This nosniff text/markdown attachment is not served as HTML. Comments are HTML-escaped; source snippets use fences longer than any embedded backtick run.
	_, _ = w.Write([]byte(application.CommentsMarkdown(code, base, head)))
}
