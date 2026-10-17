// Package server exposes only normalized IR through a loopback-local HTTP UI.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"seamgraph/internal/config"
	"seamgraph/internal/gitdiff"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"seamgraph/internal/ui"
	"seamgraph/internal/verification"
)

type Model struct {
	Snapshot  ir.Snapshot         `json:"snapshot"`
	Check     policy.Result       `json:"check"`
	Contracts verification.Report `json:"contracts"`
	Policy    policy.Policy       `json:"policy"`
}
type Load func(context.Context) (Model, error)
type Compare func(context.Context, string, string) (gitdiff.Result, error)
type App struct {
	Root      string
	Load      Load
	Compare   Compare
	token     string
	mu        sync.Mutex
	proposals map[string]config.Edit
}

func New(root string, load Load, compare Compare) (*App, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	token := make([]byte, 32)
	if _, err = rand.Read(token); err != nil {
		return nil, err
	}
	return &App{Root: root, Load: load, Compare: compare, token: hex.EncodeToString(token), proposals: map[string]config.Edit{}}, nil
}
func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), 500)
	}
}
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	assets, _ := fs.Sub(ui.Assets, "assets")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { render(w, r, ui.Page(a.token)) })
	load := func(w http.ResponseWriter, r *http.Request) (Model, bool) {
		m, err := a.Load(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return m, false
		}
		return m, true
	}
	mux.HandleFunc("GET /api/model", func(w http.ResponseWriter, r *http.Request) {
		m, ok := load(w, r)
		if !ok {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m)
	})
	mux.HandleFunc("GET /module", func(w http.ResponseWriter, r *http.Request) {
		m, ok := load(w, r)
		if !ok {
			return
		}
		for _, module := range m.Snapshot.Modules {
			if module.ID == r.URL.Query().Get("id") {
				render(w, r, ui.ModuleDetails(module, m.Snapshot, m.Check, m.Contracts))
				return
			}
		}
		http.Error(w, "unknown module ID", 404)
	})
	mux.HandleFunc("GET /edge", func(w http.ResponseWriter, r *http.Request) {
		m, ok := load(w, r)
		if !ok {
			return
		}
		from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
		relations := []ir.Relation{}
		owners := ir.Owners(m.Snapshot)
		for _, rel := range m.Snapshot.Relations {
			if owners[rel.FromID] == from && owners[rel.ToID] == to {
				relations = append(relations, rel)
			}
		}
		if len(relations) == 0 {
			http.Error(w, "unknown relationship", 404)
			return
		}
		render(w, r, ui.Relationship(relations, m.Check, a.token, from, to))
	})
	mux.HandleFunc("GET /rules", func(w http.ResponseWriter, r *http.Request) {
		m, ok := load(w, r)
		if ok {
			render(w, r, ui.Rules(m.Policy))
		}
	})
	mux.HandleFunc("GET /source", func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		if !ir.ValidPath(file) {
			http.Error(w, "invalid source location", 400)
			return
		}
		m, ok := load(w, r)
		if !ok {
			return
		}
		known := false
		for _, rel := range m.Snapshot.Relations {
			for _, e := range rel.Evidence {
				if e.File == file {
					known = true
				}
			}
		}
		for _, d := range m.Snapshot.Diagnostics {
			for _, e := range d.Evidence {
				if e.File == file {
					known = true
				}
			}
		}
		for _, e := range m.Contracts.Evidence {
			if e.SourceLocation != nil && e.SourceLocation.File == file {
				known = true
			}
		}
		if !known {
			http.Error(w, "source is not referenced by analyzed evidence", 404)
			return
		}
		real, err := filepath.EvalSymlinks(filepath.Join(a.Root, filepath.FromSlash(file)))
		if err != nil {
			http.Error(w, "source unavailable", 404)
			return
		}
		rel, err := filepath.Rel(a.Root, real)
		if err != nil || !ir.ValidPath(filepath.ToSlash(rel)) {
			http.Error(w, "source escapes project", 403)
			return
		}
		info, err := os.Stat(real)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 2*1024*1024 {
			http.Error(w, "source unavailable or too large", 400)
			return
		}
		b, err := os.ReadFile(real)
		if err != nil {
			http.Error(w, "source unavailable", 404)
			return
		}
		lines := strings.Split(string(b), "\n")
		var numbered strings.Builder
		for i, line := range lines {
			fmt.Fprintf(&numbered, "%5d  %s\n", i+1, line)
		}
		render(w, r, ui.Source(file, numbered.String()))
	})
	mux.HandleFunc("GET /changes", func(w http.ResponseWriter, r *http.Request) {
		if a.Compare == nil {
			http.Error(w, "revision comparison unavailable", 503)
			return
		}
		result, err := a.Compare(r.Context(), r.URL.Query().Get("base"), r.URL.Query().Get("head"))
		if err != nil {
			render(w, r, ui.Message(err.Error()))
			return
		}
		render(w, r, ui.Changes(result))
	})
	validate := func(w http.ResponseWriter, r *http.Request) bool {
		r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", 400)
			return false
		}
		if r.Form.Get("token") != a.token {
			http.Error(w, "invalid confirmation token", 403)
			return false
		}
		origin := r.Header.Get("Origin")
		if origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host || u.Scheme != "http" {
				http.Error(w, "cross-origin writes denied", 403)
				return false
			}
		}
		return true
	}
	mux.HandleFunc("POST /rules/propose", func(w http.ResponseWriter, r *http.Request) {
		if !validate(w, r) {
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		e, err := config.PrepareRule(a.Root, r.Form.Get("effect"), r.Form.Get("from"), r.Form.Get("to"))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if len(a.proposals) >= 100 {
			a.proposals = map[string]config.Edit{}
		}
		key := make([]byte, 24)
		if _, err = rand.Read(key); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		id := hex.EncodeToString(key)
		a.proposals[id] = e
		render(w, r, ui.Proposal(e.Diff, a.token, id))
	})
	mux.HandleFunc("POST /rules/cancel", func(w http.ResponseWriter, r *http.Request) {
		if !validate(w, r) {
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		delete(a.proposals, r.Form.Get("proposal"))
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /rules/confirm", func(w http.ResponseWriter, r *http.Request) {
		if !validate(w, r) {
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		id := r.Form.Get("proposal")
		e, ok := a.proposals[id]
		delete(a.proposals, id)
		if !ok {
			http.Error(w, "unknown/expired proposal", 400)
			return
		}
		if err := e.Commit(a.Root); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		render(w, r, ui.Message("Rule saved. Reload the architecture to refresh policy results. Backup: seamgraph.yaml.bak"))
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			http.Error(w, "loopback host required", 403)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}
func validateListenAddress(addr string, allowNetwork bool) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if !allowNetwork && host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("server address must be loopback (use --allow-network for container port publishing)")
	}
	if _, err = strconv.Atoi(port); err != nil {
		return err
	}
	return nil
}

// Serve accepts a non-loopback bind only when explicitly enabled for container
// networking. The handler retains localhost Host, Origin and CSRF validation.
func Serve(ctx context.Context, addr string, handler http.Handler, allowNetwork bool) error {
	if err := validateListenAddress(addr, allowNetwork); err != nil {
		return err
	}
	s := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 2 * time.Minute, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(shutdown)
	}()
	err := s.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
