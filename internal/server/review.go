package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"seamgraph/internal/application"
	"seamgraph/internal/ui"
)

func (a *App) review(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if query.Get("scope") == "branches" || query.Get("scope") == "target-tip" {
		_, base := comparisonRefs(r)
		query.Set("base", base)
	} else if !strings.Contains(query.Get("base"), ":") && query.Get("scope") != "revisions" && query.Get("head") != "WORKTREE" {
		base := query.Get("base")
		if base == "" {
			base = "HEAD"
		}
		query.Set("base", "merge-base:"+base)
	}
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
		if query.Get("tab") == "" {
			query.Set("tab", "hotspots")
		}
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
	result.Options = reviewOptions(query)
	result.PathFilter = query.Get("q")
	result.TypeFilter = query.Get("type")
	result.Summary = application.SummarizeReview(result)
	switch query.Get("export") {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="seamgraph-review.json"`)
		_ = json.NewEncoder(w).Encode(result)
	case "markdown":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="seamgraph-review.md"`)
		// #nosec G705 -- This text/markdown attachment is nosniff, not HTML; embedded source and notes are JSON-escaped.
		_, _ = fmt.Fprint(w, application.ReviewMarkdown(result))
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
	entry.Checklist = r.Form.Get("checklist") == "1"
	entry.Comment = r.Form.Get("comment") == "1"
	entry.Deleted = entry.Comment && r.Form.Get("deleteComment") == "1"
	entry.CommentID = r.Form.Get("commentId")
	entry.EditVersion = r.Form.Get("commentVersion")
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
	if entry.Comment && r.Header.Get("X-SeamGraph-Comment") == "1" {
		code, err := a.backend.Code(r.Context(), entry.HeadRef, entry.BaseRef, r.Form.Get("selectedFile"), "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		render(w, r, ui.CommentUpdates(code))
		return
	}
	query := url.Values{"tools": {"1"}, "base": {entry.BaseRef}, "revision": {entry.HeadRef}, "file": {entry.File}}
	tab := "review"
	if entry.Comment {
		tab = "source"
	}
	query.Set("tab", tab)
	destination := "/explore?" + query.Encode()
	if entry.Comment {
		destination = commentReturn(r, destination)
	}
	// #nosec G710 -- commentReturn restricts the path and origin to the fixed local /explore route and re-encodes its query.
	http.Redirect(w, r, destination, http.StatusSeeOther)
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

func commentReturn(r *http.Request, fallback string) string {
	address, err := url.Parse(r.Form.Get("returnTo"))
	if err != nil || address.Scheme != "" || address.Host != "" || address.Path != "/explore" {
		return fallback
	}
	return "/explore?" + address.Query().Encode()
}
