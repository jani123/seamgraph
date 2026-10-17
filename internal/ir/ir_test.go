package ir

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) Snapshot {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../../testdata/ir", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestSharedArchitectureContract(t *testing.T) {
	golden, err := os.ReadFile("../../testdata/ir/architecture.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go", "typescript", "python"} {
		t.Run(name, func(t *testing.T) {
			s := fixture(t, name)
			got, err := s.ArchitectureJSON()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, golden) {
				t.Fatalf("architecture differs:\n%s", got)
			}
			b, err := s.JSON()
			if err != nil {
				t.Fatal(err)
			}
			var round Snapshot
			if err := json.Unmarshal(b, &round); err != nil {
				t.Fatal(err)
			}
			again, err := round.JSON()
			if err != nil || !bytes.Equal(b, again) {
				t.Fatal("canonical round trip unstable", err)
			}
		})
	}
}
func TestGoldenSnapshot(t *testing.T) {
	s := fixture(t, "go")
	got, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/ir/snapshot.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("snapshot differs:\n%s", got)
	}
	// Reorder collections and duplicate evidence without changing canonical output.
	s.Modules[0], s.Modules[1] = s.Modules[1], s.Modules[0]
	s.Relations[0].Evidence = append(s.Relations[0].Evidence, s.Relations[0].Evidence...)
	b, err := s.JSON()
	if err != nil || !bytes.Equal(got, b) {
		t.Fatal("ordering/evidence dedup unstable", err)
	}
}
func TestInvalidGraphs(t *testing.T) {
	tests := map[string]func(*Snapshot){
		"version":         func(s *Snapshot) { s.SchemaVersion = 2 },
		"duplicate":       func(s *Snapshot) { s.Modules = append(s.Modules, s.Modules[0]) },
		"module ID":       func(s *Snapshot) { s.Modules[0].ID = "module:a.b" },
		"unknown module":  func(s *Snapshot) { s.Seams[0].ModuleID = "module:missing" },
		"unknown seam":    func(s *Snapshot) { s.Modules[1].Seams = append(s.Modules[1].Seams, "seam:world.missing") },
		"unknown member":  func(s *Snapshot) { s.Seams[0].ContractMembers = []string{"symbol:missing:x"} },
		"private member":  func(s *Snapshot) { s.Symbols[0].Visibility = "private" },
		"symbol identity": func(s *Snapshot) { s.Symbols[0].ID = "symbol:other:x" },
		"edge ID":         func(s *Snapshot) { s.Relations[0].ID = "edge:random" },
		"endpoint": func(s *Snapshot) {
			s.Relations[0].ToID = "module:missing"
			r := &s.Relations[0]
			r.ID = RelationID(r.Kind, r.FromID, r.ToID)
		},
		"no evidence":     func(s *Snapshot) { s.Relations[0].Evidence = nil },
		"unresolved fact": func(s *Snapshot) { s.Relations[0].Evidence[0].Resolution = "unresolved" },
		"escape":          func(s *Snapshot) { s.Relations[0].Evidence[0].File = "../secret" },
		"line":            func(s *Snapshot) { s.Relations[0].Evidence[0].StartLine = 0 },
		"confidence":      func(s *Snapshot) { s.Relations[0].Evidence[0].Confidence = "certain" },
		"diagnostic ref": func(s *Snapshot) {
			s.Diagnostics = []Diagnostic{{Code: "X", Message: "unknown", Severity: "warning", EntityIDs: []string{"absent"}}}
		},
	}
	for n, mutate := range tests {
		t.Run(n, func(t *testing.T) {
			s := fixture(t, "go")
			mutate(&s)
			if err := s.Validate(); err == nil {
				t.Fatal("invalid graph accepted")
			}
		})
	}
}
func TestMerge(t *testing.T) {
	s := fixture(t, "go")
	p := PartialSnapshot{Modules: s.Modules, Seams: s.Seams, Symbols: s.Symbols, Relations: s.Relations}
	got, err := Merge(p, p)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := got.JSON()
	want, _ := s.JSON()
	if !bytes.Equal(b, want) {
		t.Fatal("merge changed graph")
	}
	p.Modules = append([]Module{}, p.Modules...)
	p.Modules[0].DisplayName = "conflict"
	if _, err := Merge(PartialSnapshot{Modules: s.Modules}, p); err == nil {
		t.Fatal("conflicting definition accepted")
	}
	if _, err := Merge(PartialSnapshot{Relations: s.Relations}); err == nil {
		t.Fatal("dangling references accepted")
	}
}
func TestIdentityPolicy(t *testing.T) {
	id := SymbolID("world", "api/Read: space%")
	if !strings.Contains(id, "%") || id == SymbolID("world", "api/Read: space") {
		t.Fatal("symbol escaping collision")
	}
	if ModuleID("world") == ModuleID("renamed") {
		t.Fatal("rename must change identity")
	}
	s := fixture(t, "go")
	r := s.Relations[0]
	s.Relations[0].Evidence[0].StartLine = 2
	s.Relations[0].Evidence[0].EndLine = 2
	s.Symbols[0].DisplaySignature = "Read(id string) Value"
	if s.Relations[0].ID != r.ID {
		t.Fatal("signature/location changed identity")
	}
	if ViolationID("a", r.ID) == ViolationID("b", r.ID) {
		t.Fatal("violation identity collision")
	}
}
func TestCanonicalDoesNotMutate(t *testing.T) {
	s := fixture(t, "go")
	s.Modules[1].SourceRoots = []string{"z", "a"}
	before, _ := json.Marshal(s)
	if _, err := s.JSON(); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(s)
	if !bytes.Equal(before, after) {
		t.Fatal("canonical mutated caller")
	}
}

func TestDecode(t *testing.T) {
	s := fixture(t, "go")
	b, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(b); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{append(append([]byte{}, b...), []byte("{}")...), []byte(strings.Replace(string(b), `"schemaVersion": 1`, `"schemaVersion": 1, "unknown": true`, 1)), []byte(`{"schemaVersion":2}`)} {
		if _, err := Decode(bad); err == nil {
			t.Fatal("invalid serialized IR accepted")
		}
	}
}
func TestOverlappingIRRoots(t *testing.T) {
	s := fixture(t, "go")
	s.Modules[0].SourceRoots = []string{"src/world/nested"}
	if err := s.Validate(); err == nil {
		t.Fatal("ambiguous ownership accepted")
	}
}

func TestMergePreservesBypassAcrossAccessPaths(t *testing.T) {
	s := fixture(t, "go")
	other := s.Relations[0]
	other.BypassesSeam = true
	merged, err := Merge(PartialSnapshot{Modules: s.Modules, Seams: s.Seams, Symbols: s.Symbols, Relations: s.Relations}, PartialSnapshot{Relations: []Relation{other}})
	if err != nil || !merged.Relations[0].BypassesSeam || len(merged.Relations[0].BypassEvidence) == 0 {
		t.Fatal("bypass lost across mixed access paths", err)
	}
}
