package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"seamgraph/internal/application"
	"seamgraph/internal/bootstrap"
	"seamgraph/internal/projectfiles"
	"strconv"
	"strings"
	"testing"
	"time"

	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"seamgraph/internal/verification"
)

func fixtureApp(t *testing.T) *App {
	t.Helper()
	b, err := os.ReadFile("../../testdata/ir/go.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := ir.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cfg := `version: 1
modules:
  npc:
    language: go
    paths: [src/npc]
  world:
    language: go
    paths: [src/world]
    seams:
      reader:
        symbols: [Read]
policies:
  default: deny
`
	if err = os.WriteFile(filepath.Join(root, "seamgraph.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(root, "src/npc"), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "src/npc/main.go"), []byte("package npc\n// <unsafe>\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p := policy.Policy{Default: "deny", Unresolved: "error"}
	r, err := policy.Evaluate(s, p)
	if err != nil {
		t.Fatal(err)
	}
	service, err := bootstrap.New(root)
	if err != nil {
		t.Fatal(err)
	}
	backend := &testExplorer{Service: service, load: func(context.Context) (application.Model, error) {
		return application.Model{Snapshot: s, Check: r, Policy: p, Contracts: verification.Report{SchemaVersion: 1, Results: []verification.Result{}}}, nil
	}}
	a, err := New(backend)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func TestExplorerHTTP(t *testing.T) {
	a := fixtureApp(t)
	for _, path := range []string{"/", "/help", "/api/model", "/module?id=module:world", "/edge?from=module:npc&to=module:world", "/rules", "/assets/app.js", "/assets/styles.css", "/source?file=src/npc/main.go"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://127.0.0.1"+path, nil)
			w := httptest.NewRecorder()
			a.Handler().ServeHTTP(w, req)
			if w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			}
			if path == "/" && (strings.Contains(w.Body.String(), "https://") || !strings.Contains(w.Body.String(), "/assets/cytoscape.min.js")) {
				t.Fatal("missing local assets")
			}
			if strings.HasPrefix(path, "/module") && (!strings.Contains(w.Body.String(), "Copy ID") || !strings.Contains(w.Body.String(), "seam:world.reader") || !strings.Contains(w.Body.String(), "module:npc")) {
				t.Fatal("missing seam/consumer details")
			}
			if strings.HasPrefix(path, "/source") && strings.Contains(w.Body.String(), "<unsafe>") {
				t.Fatal("unescaped source")
			}
		})
	}
	for _, path := range []string{"/source?file=../secret", "/source?file=seamgraph.yaml", "/module?id=module:absent"} {
		req := httptest.NewRequest("GET", "http://127.0.0.1"+path, nil)
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, req)
		if w.Code == 200 {
			t.Fatal("unsafe/unknown access accepted", path)
		}
	}
	req := httptest.NewRequest("GET", "http://attacker.example/", nil)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("DNS rebinding host accepted")
	}
}
func TestConfirmedRulesAndConcurrentChanges(t *testing.T) {
	a := fixtureApp(t)
	request := func(path string, values url.Values, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "http://127.0.0.1"+path, strings.NewReader(values.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, req)
		return w
	}
	values := url.Values{"token": {a.token}, "from": {"module:npc"}, "to": {"seam:world.reader"}, "effect": {"allow"}}
	original, _ := os.ReadFile(filepath.Join(a.backend.(*testExplorer).Root(), "seamgraph.yaml"))
	w := request("/rules/propose", values, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	after, _ := os.ReadFile(filepath.Join(a.backend.(*testExplorer).Root(), "seamgraph.yaml"))
	if string(original) != string(after) {
		t.Fatal("preview modified file")
	}
	proposal := ""
	proposal = proposalID(t, w.Body.String())
	if proposal == "" {
		t.Fatal("no confirmation proposal")
	}
	wrong := request("/rules/confirm", url.Values{"token": {"wrong"}, "proposal": {proposal}}, "")
	if wrong.Code != 403 {
		t.Fatal("missing CSRF rejection")
	}
	wrong = request("/rules/confirm", url.Values{"token": {a.token}, "proposal": {proposal}}, "http://evil.example")
	if wrong.Code != 403 {
		t.Fatal("origin accepted")
	}
	w = request("/rules/confirm", url.Values{"token": {a.token}, "proposal": {proposal}}, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(a.backend.(*testExplorer).Root(), "seamgraph.yaml.bak")); err != nil {
		t.Fatal(err)
	}
	values.Set("from", "module:world")
	values.Set("to", "module:npc")
	if w = request("/rules/propose", values, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	proposal = proposalID(t, w.Body.String())
	f, err := os.OpenFile(filepath.Join(a.backend.(*testExplorer).Root(), "seamgraph.yaml"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("# concurrent change\n")
	f.Close()
	w = request("/rules/confirm", url.Values{"token": {a.token}, "proposal": {proposal}}, "")
	if w.Code != 409 {
		t.Fatal("concurrent config overwritten", w.Code)
	}
	values.Set("to", "module:unknown")
	if w = request("/rules/propose", values, ""); w.Code != 400 {
		t.Fatal("invalid config proposal allowed")
	}
}
func TestLargeModel(t *testing.T) {
	a := fixtureApp(t)
	m, _ := a.backend.Load(context.Background())
	s := ir.Snapshot{SchemaVersion: 1}
	for i := 0; i < 100; i++ {
		name := fmtInt(i)
		s.Modules = append(s.Modules, ir.Module{ID: ir.ModuleID(name), DisplayName: name, SourceRoots: []string{"src/" + name}})
	}
	for i := 0; i < 100; i++ {
		for j := 1; j <= 5; j++ {
			from, to := s.Modules[i].ID, s.Modules[(i+j)%100].ID
			s.Relations = append(s.Relations, ir.Relation{ID: ir.RelationID("imports", from, to), FromID: from, ToID: to, Kind: "imports", Evidence: m.Snapshot.Relations[0].Evidence})
		}
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	a.backend.(*testExplorer).load = func(context.Context) (application.Model, error) { return application.Model{Snapshot: s}, nil }
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1/api/model", nil))
	var got application.Model
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got.Snapshot.Modules) != 100 || len(got.Snapshot.Relations) != 500 {
		t.Fatal("large model lost data", err)
	}
}
func fmtInt(i int) string { return "m" + strconv.Itoa(i) }

func TestCanceledProposalCannotBeConfirmed(t *testing.T) {
	a := fixtureApp(t)
	post := func(path string, values url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "http://127.0.0.1"+path, strings.NewReader(values.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, req)
		return w
	}
	original, _ := os.ReadFile(filepath.Join(a.backend.(*testExplorer).Root(), "seamgraph.yaml"))
	w := post("/rules/propose", url.Values{"token": {a.token}, "effect": {"allow"}, "from": {"module:npc"}, "to": {"seam:world.reader"}})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	id := ""
	id = proposalID(t, w.Body.String())
	values := url.Values{"token": {a.token}, "proposal": {id}}
	if w := post("/rules/cancel", values); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := post("/rules/confirm", values); w.Code != 400 {
		t.Fatal("canceled proposal still confirmable", w.Code)
	}
	after, _ := os.ReadFile(filepath.Join(a.backend.(*testExplorer).Root(), "seamgraph.yaml"))
	if string(original) != string(after) {
		t.Fatal("canceled rule wrote configuration")
	}
}

func TestContainerBindIsExplicit(t *testing.T) {
	for _, tc := range []struct {
		addr      string
		allow, ok bool
	}{
		{"127.0.0.1:8765", false, true},
		{"[::1]:8765", false, true},
		{"0.0.0.0:8765", false, false},
		{"0.0.0.0:8765", true, true},
		{"[::]:8765", true, true},
		{"invalid", true, false},
	} {
		if err := validateListenAddress(tc.addr, tc.allow); (err == nil) != tc.ok {
			t.Errorf("%s allow=%t: %v", tc.addr, tc.allow, err)
		}
	}
	// Enabling container binding never changes the browser Host boundary.
	w := httptest.NewRecorder()
	fixtureApp(t).Handler().ServeHTTP(w, httptest.NewRequest("GET", "http://untrusted.example/", nil))
	if w.Code != 403 {
		t.Fatalf("non-local Host accepted: %d", w.Code)
	}
}

func TestEphemeralPortAnnouncement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	announced := make(chan string, 1)
	finished := make(chan error, 1)
	go func() {
		finished <- Serve(ctx, "127.0.0.1:0", fixtureHandler(), false, func(bound string) error { announced <- bound; return nil })
	}()
	var addr string
	select {
	case addr = <-announced:
	case err := <-finished:
		t.Fatalf("server stopped before listening: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not announce its port")
	}
	if strings.HasSuffix(addr, ":0") {
		t.Fatal("announced requested address instead of actual port")
	}
	response, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}
func fixtureHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
}

type testExplorer struct {
	*application.Service
	load func(context.Context) (application.Model, error)
}

func (e *testExplorer) Load(ctx context.Context) (application.Model, error) { return e.load(ctx) }
func (e *testExplorer) Source(ctx context.Context, file string) (string, error) {
	m, err := e.load(ctx)
	if err != nil {
		return "", err
	}
	locations := []ir.Evidence{}
	for _, relation := range m.Snapshot.Relations {
		locations = append(locations, relation.Evidence...)
	}
	return projectfiles.Source(e.Root(), file, locations)
}
func proposalID(t *testing.T, body string) string {
	t.Helper()
	matches := regexp.MustCompile(`name="proposal" value="([a-f0-9]+)"`).FindStringSubmatch(body)
	if len(matches) != 2 {
		t.Fatalf("missing proposal input: %s", body)
	}
	return matches[1]
}
