package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"seamgraph/internal/verification"
	"testing"
)

func TestCLISmoke(t *testing.T) {
	root := t.TempDir()
	var out, errs bytes.Buffer
	invoke := func(args ...string) int { out.Reset(); errs.Reset(); return run(args, &out, &errs) }
	if code := invoke("init", "--root", root); code != 0 {
		t.Fatal(code, errs.String())
	}
	if code := invoke("validate", "--root", root); code != 0 {
		t.Fatal(code, errs.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("analysis not performed")) {
		t.Fatal("validation implied analysis")
	}
	original, err := os.ReadFile(filepath.Join(root, "seamgraph.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if code := invoke("init", "--root", root); code != 2 {
		t.Fatal("init overwrote configuration")
	}
	after, _ := os.ReadFile(filepath.Join(root, "seamgraph.yaml"))
	if !bytes.Equal(original, after) {
		t.Fatal("existing config changed")
	}
	if err := os.WriteFile(filepath.Join(root, "seamgraph.yaml"), []byte("version: ["), 0600); err != nil {
		t.Fatal(err)
	}
	if code := invoke("validate", "--root", root); code != 2 {
		t.Fatal("malformed YAML passed", code)
	}
	for _, args := range [][]string{{}, {"check"}, {"scan"}, {"snapshot"}, {"validate", "--unknown"}, {"validate", "unexpected"}, {"validate", "--root", t.TempDir()}} {
		if code := invoke(args...); code != 2 {
			t.Fatalf("%v: false success %d", args, code)
		}
	}
}

func TestCLIAnalysisCommands(t *testing.T) {
	for _, language := range []string{"go", "typescript", "python"} {
		t.Run(language, func(t *testing.T) {
			root, err := filepath.Abs(filepath.Join("../../testdata/projects", language))
			if err != nil {
				t.Fatal(err)
			}
			var out, errs bytes.Buffer
			for _, args := range [][]string{{"scan", "--root", root, "--json"}, {"check", "--root", root, "--json"}, {"check", "--root", root, "--format", "sarif"}, {"inspect", "seam:world.reader", "--root", root, "--tests", "--json"}, {"check", "--root", root, "--contracts", "--json"}, {"snapshot", "--root", root, "--output", filepath.Join(t.TempDir(), "snapshot.json")}} {
				out.Reset()
				errs.Reset()
				if code := run(args, &out, &errs); code != 0 {
					t.Fatal(args, code, errs.String(), out.String())
				}
			}
			out.Reset()
			errs.Reset()
			if code := run([]string{"inspect", "seam:absent", "--root", root, "--json"}, &out, &errs); code != 2 || !bytes.Contains(errs.Bytes(), []byte("candidate IDs")) {
				t.Fatal("unknown ID did not fail usefully", code, errs.String())
			}
		})
	}
}
func TestActualContractCollectionAndFreshness(t *testing.T) {
	root := t.TempDir()
	source := "../../testdata/projects/go"
	if err := filepath.WalkDir(source, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(source, p)
		target := filepath.Join(root, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0644)
	}); err != nil {
		t.Fatal(err)
	}
	testSource := []byte("package world\nimport \"testing\"\nfunc TestOpen(t *testing.T){if got:=Open().Read();got!=\"world\"{t.Fatalf(\"unexpected world: %s\",got)}}\n")
	if err := os.WriteFile(filepath.Join(root, "world/world_test.go"), testSource, 0644); err != nil {
		t.Fatal(err)
	}
	manifest := `version: 1
requirements:
  - id: behavior:open-result
    seamId: seam:world.reader
    description: Open returns a reader whose Read result is world
    kind: behavior
links:
  - testId: example.test/architecture/world/TestOpen
    requirementIds: [behavior:open-result]
    memberIds: []
    reviewed: true
policies:
  seam:world.reader: require-verified
`
	if err := os.WriteFile(filepath.Join(root, "seamgraph-contracts.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "contract fixture"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(err, string(b))
		}
	}
	var out, errs bytes.Buffer
	if code := run([]string{"test", "--runner", "go", "--root", root}, &out, &errs); code != 0 {
		t.Fatal("runner failed", code, errs.String())
	}
	out.Reset()
	errs.Reset()
	if code := run([]string{"check", "--contracts", "--json", "--root", root}, &out, &errs); code != 0 || !bytes.Contains(out.Bytes(), []byte(`"status": "verified"`)) {
		t.Fatal("passing explicit contract not verified", code, errs.String(), out.String())
	}
	if err := os.WriteFile(filepath.Join(root, "world/world.go"), append(mustRead(t, filepath.Join(root, "world/world.go")), []byte("\n// implementation changed\n")...), 0644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errs.Reset()
	if code := run([]string{"check", "--contracts", "--json", "--root", root}, &out, &errs); code != 2 || !bytes.Contains(out.Bytes(), []byte(`"status": "stale"`)) {
		t.Fatal("changed implementation falsely current", code, errs.String(), out.String())
	}
}
func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestEvidenceStoreSerializesAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	e := verification.Evidence{ID: "e:one", TestID: "Read", Runner: "test", Revision: "abc", Fingerprint: "inputs", RunID: "run", ObservedAt: "2026-10-16T00:00:00Z", Outcome: "passed", EvidenceKind: "test-execution"}
	if _, err := appendEvidence(root, []verification.Evidence{e}); err != nil {
		t.Fatal(err)
	}
	s, err := appendEvidence(root, []verification.Evidence{e})
	if err != nil || len(s.Evidence) != 1 {
		t.Fatal("same evidence not idempotent", err, s)
	}
	lock := filepath.Join(root, ".seamgraph/evidence.lock")
	if err = os.WriteFile(lock, []byte("other writer"), 0600); err != nil {
		t.Fatal(err)
	}
	e.ID = "e:two"
	if _, err = appendEvidence(root, []verification.Evidence{e}); err == nil {
		t.Fatal("concurrent writer bypassed lock")
	}
	s, err = readStore(root)
	if err != nil || len(s.Evidence) != 1 {
		t.Fatal("lock failure changed evidence", err, s)
	}
}
