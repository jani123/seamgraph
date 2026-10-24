package server

import (
	"encoding/json"
	"net/http"

	"seamgraph/internal/ui"
)

func (a *App) code(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	browser, err := a.backend.Code(r.Context(), query.Get("revision"), query.Get("base"), query.Get("file"), query.Get("object"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if query.Get("format") == "json" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(browser)
		return
	}
	filter := ui.CodeFilter{Query: query.Get("q"), Kind: query.Get("kind"), Visibility: query.Get("visibility"), Status: query.Get("status"), View: query.Get("view")}
	render(w, r, ui.CodePage(browser, filter))
}
