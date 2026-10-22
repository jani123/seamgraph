package application

import (
	"seamgraph/internal/ir"
	"seamgraph/internal/snapshot"
	"testing"
)

func TestFileAttributionAndRenames(t *testing.T) {
	before := map[string]string{"a/private.go": "old", "a/move.go": "move", "a/gone.go": "gone", "seamgraph.yaml": "one"}
	after := map[string]string{"a/private.go": "new", "b/moved.go": "move", "b/new.py": "new", "seamgraph.yaml": "two"}
	files := fileChanges(before, after)
	if len(files) != 5 {
		t.Fatal(files)
	}
	graph := ir.Snapshot{Modules: []ir.Module{{ID: "module:a", SourceRoots: []string{"a"}}, {ID: "module:b", SourceRoots: []string{"b"}}}}
	changes := moduleChanges(graph, graph, files, nil)
	if len(changes) != 2 || len(changes[0].Files) != 3 || len(changes[1].Files) != 2 {
		t.Fatal(changes)
	}
	if changes[0].Files[2].OldPath != "a/move.go" {
		t.Fatal("move must mark old and new owners", changes)
	}
	for _, m := range changes {
		if m.Status != "modified" || len(m.Seams) > 0 {
			t.Fatal("internal edits must not claim seam changes", m)
		}
	}
}
func TestConfigurationAndSeamsAreIndependentFromSourceChanges(t *testing.T) {
	graph := ir.Snapshot{Modules: []ir.Module{{ID: "module:a", SourceRoots: []string{"."}}},
		Seams:   []ir.Seam{{ID: "seam:a.api", ModuleID: "module:a", ContractMembers: []string{"symbol:a:F"}}},
		Symbols: []ir.Symbol{{ID: "symbol:a:F", ModuleID: "module:a"}, {ID: "symbol:a:Private", ModuleID: "module:a"}}}
	files := []FileChange{{Path: "seamgraph.yaml", Status: "modified"}}
	if got := moduleChanges(graph, graph, files, nil); len(got) != 0 {
		t.Fatal("configuration is not module source", got)
	}
	architecture := []snapshot.Change{{Kind: "symbol", EntityID: "symbol:a:F", Status: "modified"}, {Kind: "symbol", EntityID: "symbol:a:Private", Status: "modified"}, {Kind: "module", EntityID: "module:a", Status: "modified"}}
	got := moduleChanges(graph, graph, files, architecture)
	if len(got) != 1 || len(got[0].Files) != 0 || len(got[0].Seams) != 1 || !got[0].DefinitionChanged {
		t.Fatal(got)
	}
	empty := ir.Snapshot{}
	if got := moduleChanges(graph, empty, nil, nil); len(got) != 1 || got[0].Status != "removed" {
		t.Fatal(got)
	}
	if got := moduleChanges(empty, graph, nil, nil); len(got) != 1 || got[0].Status != "added" {
		t.Fatal(got)
	}
}

func TestPolicyChangesIdentifyAffectedModulesWithoutClaimingSourceEdits(t *testing.T) {
	graph := ir.Snapshot{Modules: []ir.Module{{ID: "module:a"}, {ID: "module:b"}}}
	change := snapshot.Change{Kind: "violation", Status: "added", After: []byte(`{"consumerId":"module:a","providerId":"module:b"}`)}
	got := moduleChanges(graph, graph, nil, []snapshot.Change{change})
	if len(got) != 2 {
		t.Fatal(got)
	}
	for _, m := range got {
		if !m.PolicyChanged || len(m.Files) != 0 {
			t.Fatal(m)
		}
	}
}
