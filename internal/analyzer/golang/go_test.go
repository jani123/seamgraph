package golang

import (
	"context"
	"os"
	"path/filepath"
	"seamgraph/internal/application"
	"seamgraph/internal/projectfiles"
	"testing"

	analyzer "seamgraph/internal/analysis"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
)

func TestCompilerResolvedArchitecture(t *testing.T) {
	root, err := filepath.Abs("../../../testdata/projects/go")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := projectfiles.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	partial, err := (Adapter{}).Analyze(context.Background(), analyzer.Project{Root: root}, doc.Config)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := ir.Merge(partial)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := application.NormalizePolicy(doc.Config)
	r, err := policy.Evaluate(snapshot, p)
	if err != nil {
		t.Fatal(err)
	}
	if r.ExitCode() != 0 {
		t.Fatalf("permitted interface reference violated: %+v", r)
	}
	implements := 0
	for _, r := range snapshot.Relations {
		if r.Kind == "implements" {
			implements++
		}
	}
	if implements == 0 {
		t.Fatal("missing structural implementation fact")
	}
	if len(snapshot.Seams[0].ContractMembers) < 3 {
		t.Fatal("interface methods not in contract", snapshot.Seams)
	}
}
func TestConcreteBypassAndLoadErrors(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"world", "npc"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"go.mod", "seamgraph.yaml", "world/world.go", "npc/npc.go"} {
		b, err := os.ReadFile(filepath.Join("../../../testdata/projects/go", file))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, file), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "npc/npc.go"), []byte("package npc\nimport w \"example.test/architecture/world\"\nvar Bad w.Concrete\n"), 0644); err != nil {
		t.Fatal(err)
	}
	doc, _ := projectfiles.Load(root)
	partial, err := (Adapter{}).Analyze(context.Background(), analyzer.Project{Root: root}, doc.Config)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ir.Merge(partial)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := application.NormalizePolicy(doc.Config)
	r, err := policy.Evaluate(s, p)
	if err != nil || r.ExitCode() != 1 || r.Violations[0].Code != "SEAM_BYPASS" {
		t.Fatal(r, err)
	}
	if err := os.WriteFile(filepath.Join(root, "world/cycle.go"), []byte("package world\nimport _ \"example.test/architecture/npc\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = (Adapter{}).Analyze(context.Background(), analyzer.Project{Root: root}, doc.Config); err == nil {
		t.Fatal("cycle silently accepted")
	}
}
