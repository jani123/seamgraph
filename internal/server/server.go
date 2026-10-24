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
	"strconv"
	"time"

	"errors"
	"github.com/a-h/templ"
	"seamgraph/internal/application"
	"seamgraph/internal/ir"
	"seamgraph/internal/ui"
)

type Explorer interface {
	Review(context.Context, string, string, string, string) (application.ReviewWorkspace, error)
	RecordReview(context.Context, application.ReviewEntry) error
	Load(context.Context) (application.Model, error)
	Compare(context.Context, string, string) (application.Result, error)
	Source(context.Context, string) (string, error)
	Propose(string, string, string) (application.Proposal, error)
	Cancel(string)
	Confirm(string) error
}
type App struct {
	backend Explorer
	token   string
}

func New(backend Explorer) (*App, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return nil, err
	}
	return &App{backend: backend, token: hex.EncodeToString(token)}, nil
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
	mux.HandleFunc("GET /review", a.review)
	mux.HandleFunc("POST /review", a.recordReview)
	mux.HandleFunc("GET /help", func(w http.ResponseWriter, r *http.Request) { render(w, r, ui.Help()) })

	mux.HandleFunc("GET /api/model", a.model)
	mux.HandleFunc("GET /module", a.module)
	mux.HandleFunc("GET /edge", a.edge)
	mux.HandleFunc("GET /rules", a.rules)
	mux.HandleFunc("GET /source", a.source)
	mux.HandleFunc("GET /changes", a.changes)
	mux.HandleFunc("POST /rules/propose", a.propose)
	mux.HandleFunc("POST /rules/cancel", a.cancel)
	mux.HandleFunc("POST /rules/confirm", a.confirm)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			http.Error(w, "loopback host required", http.StatusForbidden)
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
func Serve(ctx context.Context, addr string, handler http.Handler, allowNetwork bool, announce func(string) error) error {
	if err := validateListenAddress(addr, allowNetwork); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()
	if announce != nil {
		if err = announce(listener.Addr().String()); err != nil {
			return err
		}
	}
	s := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 2 * time.Minute, IdleTimeout: 60 * time.Second}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
		case <-done:
			return
		}
		shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := s.Shutdown(shutdown); err != nil {
			_ = s.Close()
		}
	}()
	err = s.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (a *App) loadModel(w http.ResponseWriter, r *http.Request) (application.Model, bool) {
	m, err := a.backend.Load(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return m, false
	}
	return m, true
}

func (a *App) validateWrite(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", 400)
		return false
	}
	if r.Form.Get("token") != a.token {
		http.Error(w, "invalid confirmation token", http.StatusForbidden)
		return false
	}
	origin := r.Header.Get("Origin")
	if origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host || u.Scheme != "http" {
			http.Error(w, "cross-origin writes denied", http.StatusForbidden)
			return false
		}
	}
	return true
}

func (a *App) model(w http.ResponseWriter, r *http.Request) {
	m, ok := a.loadModel(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(m) // The client may disconnect while receiving the response.
}

func (a *App) module(w http.ResponseWriter, r *http.Request) {
	m, ok := a.loadModel(w, r)
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
}

func (a *App) edge(w http.ResponseWriter, r *http.Request) {
	m, ok := a.loadModel(w, r)
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
}

func (a *App) rules(w http.ResponseWriter, r *http.Request) {
	m, ok := a.loadModel(w, r)
	if ok {
		render(w, r, ui.RulesPage(m.Policy))
	}
}

func (a *App) source(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	if !ir.ValidPath(file) {
		http.Error(w, "invalid source location", 400)
		return
	}
	source, err := a.backend.Source(r.Context(), file)
	if err != nil {
		code := 500
		var sourceError *application.SourceError
		if errors.As(err, &sourceError) {
			code = 404
			if sourceError.Kind == "invalid" {
				code = 400
			}
			if sourceError.Kind == "forbidden" {
				code = 403
			}
			if sourceError.Kind == "large" {
				code = 400
			}
		}
		http.Error(w, err.Error(), code)
		return
	}
	render(w, r, ui.Source(file, source))
}

func (a *App) changes(w http.ResponseWriter, r *http.Request) {
	if a.backend == nil {
		http.Error(w, "revision comparison unavailable", http.StatusServiceUnavailable)
		return
	}
	base, head := r.URL.Query().Get("base"), r.URL.Query().Get("head")
	if r.URL.Query().Get("scope") == "working" {
		base, head = "HEAD", "WORKTREE"
	}
	result, err := a.backend.Compare(r.Context(), base, head)
	if err != nil {
		render(w, r, ui.Message(err.Error()))
		return
	}
	render(w, r, ui.Changes(result))
}

func (a *App) propose(w http.ResponseWriter, r *http.Request) {
	if !a.validateWrite(w, r) {
		return
	}
	proposal, err := a.backend.Propose(r.Form.Get("effect"), r.Form.Get("from"), r.Form.Get("to"))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	render(w, r, ui.Proposal(proposal.Diff, a.token, proposal.ID))
}

func (a *App) cancel(w http.ResponseWriter, r *http.Request) {
	if !a.validateWrite(w, r) {
		return
	}
	a.backend.Cancel(r.Form.Get("proposal"))
	w.WriteHeader(http.StatusOK)
}

func (a *App) confirm(w http.ResponseWriter, r *http.Request) {
	if !a.validateWrite(w, r) {
		return
	}
	if err := a.backend.Confirm(r.Form.Get("proposal")); err != nil {
		code := 409
		if errors.Is(err, application.ErrUnknownProposal) {
			code = 400
		}
		http.Error(w, err.Error(), code)
		return
	}
	render(w, r, ui.Message("Rule saved. Reload the architecture to refresh policy results. Backup: seamgraph.yaml.bak"))
}
