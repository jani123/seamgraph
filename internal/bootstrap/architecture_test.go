package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"seamgraph/internal/application"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"strings"
	"testing"
)

// Analyze real production source and real seam declarations. This catches
// grants that synthetic rule fixtures alone cannot exercise.
func TestSelfArchitectureAndRealForbiddenReferences(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"go.mod", "go.sum", "seamgraph.yaml", "internal", "cmd", "scripts/packagezip"} {
		source := filepath.Join("../..", name)
		err := filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel("../..", path)
			if err != nil {
				return err
			}
			target := filepath.Join(root, rel)
			if d.IsDir() {
				return os.MkdirAll(target, 0755)
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, b, 0644)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	check := func() (ir.Snapshot, policy.Result) {
		t.Helper()
		s, c, err := Scan(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		p, err := application.NormalizePolicy(c)
		if err != nil {
			t.Fatal(err)
		}
		r, err := policy.Evaluate(s, p)
		if err != nil {
			t.Fatal(err)
		}
		if r.Incomplete {
			t.Fatalf("incomplete architecture: %+v", r.Diagnostics)
		}
		return s, r
	}
	snapshot, result := check()
	if len(result.Violations) > 0 {
		t.Fatalf("self architecture violations: %+v", result.Violations)
	}
	assertAcyclic(t, snapshot)
	for _, tc := range []struct{ name, dir, source, consumer string }{
		{"application-cannot-call-git", "internal/application", "package application\nimport g \"seamgraph/internal/gitworkspace\"\nvar architectureProbe = g.Extract\n", "module:application"},
		{"ui-cannot-execute-query", "internal/ui", "package ui\nimport a \"seamgraph/internal/application\"\nvar architectureProbe = (*a.Service).Load\n", "module:ui"},
		{"ui-cannot-execute-engine", "internal/ui", "package ui\nimport p \"seamgraph/internal/policy\"\nvar architectureProbe = p.Evaluate\n", "module:ui"},
		{"adapter-cannot-use-another-adapter", "internal/analyzer/golang", "package golang\nimport p \"seamgraph/internal/analyzer/python\"\nvar architectureProbe = p.Adapter{}\n", "module:go-adapter"},
		{"cli-cannot-write-project-files", "cmd/seamgraph", "package main\nimport p \"seamgraph/internal/projectfiles\"\nvar architectureProbe = p.PrepareRule\n", "module:cli"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(root, tc.dir, "architecture_probe.go")
			if err := os.WriteFile(file, []byte(tc.source), 0644); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.Remove(file) }()
			_, result := check()
			found := false
			for _, v := range result.Violations {
				found = found || v.ConsumerID == tc.consumer
			}
			if !found {
				t.Fatalf("real forbidden reference was allowed: %+v", result)
			}
		})
	}
}
func assertAcyclic(t *testing.T, s ir.Snapshot) {
	t.Helper()
	owners := ir.Owners(s)
	edges := map[string]map[string]bool{}
	for _, r := range s.Relations {
		if !ir.IsConsumerRelation(r) {
			continue
		}
		a, b := owners[r.FromID], owners[r.ToID]
		if a == "" || b == "" || a == b {
			continue
		}
		if edges[a] == nil {
			edges[a] = map[string]bool{}
		}
		edges[a][b] = true
	}
	active, done := map[string]bool{}, map[string]bool{}
	var visit func(string, []string)
	visit = func(n string, path []string) {
		if active[n] {
			t.Fatalf("module dependency cycle: %v -> %s", path, n)
		}
		if done[n] {
			return
		}
		active[n] = true
		for m := range edges[n] {
			visit(m, append(path, n))
		}
		delete(active, n)
		done[n] = true
	}
	for n := range edges {
		visit(n, nil)
	}
}
