package gitdiff

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"seamgraph/internal/analyzer/registry"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
)

func command(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(b))
	}
	return string(b)
}
func TestCommittedArchitectureAndWorkingTreeIsolation(t *testing.T) {
	root := t.TempDir()
	source := "../../testdata/projects/go"
	err := filepath.WalkDir(source, func(p string, d os.DirEntry, err error) error {
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
	})
	if err != nil {
		t.Fatal(err)
	}
	command(t, root, "init", "-q")
	command(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "add", ".")
	command(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	if err = os.WriteFile(filepath.Join(root, "npc/bad.go"), []byte("package npc\nimport w \"example.test/architecture/world\"\nvar Forbidden w.Concrete\n"), 0644); err != nil {
		t.Fatal(err)
	}
	command(t, root, "add", ".")
	command(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "forbidden edge")
	working := []byte("package npc\n// uncommitted state must survive\n")
	if err = os.WriteFile(filepath.Join(root, "npc/npc.go"), working, 0644); err != nil {
		t.Fatal(err)
	}
	analyze := func(ctx context.Context, root string) (ir.Snapshot, policy.Policy, error) {
		s, c, err := registry.Scan(ctx, root)
		if err != nil {
			return s, policy.Policy{}, err
		}
		p, err := c.Policy()
		return s, p, err
	}
	r, err := Compare(context.Background(), root, "HEAD~1", "HEAD", analyze)
	if err != nil {
		t.Fatal(err)
	}
	added := false
	for _, change := range r.Diff.Changes {
		if change.Kind == "violation" && change.Status == "added" {
			added = true
		}
	}
	if !added {
		t.Fatal("missed regression", r)
	}
	after, _ := os.ReadFile(filepath.Join(root, "npc/npc.go"))
	if string(after) != string(working) {
		t.Fatal("working tree altered")
	}
	if _, err = Revision(context.Background(), root, "--help"); err == nil {
		t.Fatal("invalid ref accepted")
	}
}

func TestSessionCacheUsesRevisionConfigAndContext(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "seamgraph.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	command(t, root, "init", "-q")
	command(t, root, "add", ".")
	command(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	calls := 0
	analyze := func(context.Context, string) (ir.Snapshot, policy.Policy, error) {
		calls++
		return ir.Snapshot{SchemaVersion: 1}, policy.Policy{Default: "allow", Unresolved: "error"}, nil
	}
	cache := &Cache{Context: "first"}
	for i := 0; i < 2; i++ {
		if _, err := cache.Compare(context.Background(), root, "HEAD", "HEAD", analyze); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatal("identical immutable snapshots not cached", calls)
	}
	cache.Context = "changed"
	if _, err := cache.Compare(context.Background(), root, "HEAD", "HEAD", analyze); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("changed analysis context reused cache", calls)
	}
}

func TestExactTreeIgnoresArchiveTransformations(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{".gitattributes": "seamgraph.yaml export-ignore\ndata.txt export-subst\n", "seamgraph.yaml": "version: 1\n", "data.txt": "$Format:%H$\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	command(t, root, "init", "-q")
	command(t, root, "add", ".")
	command(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "attributes")
	dir, _, cleanup, err := Extract(context.Background(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err = os.Stat(filepath.Join(dir, "seamgraph.yaml")); err != nil {
		t.Fatal("archive attribute hid config", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "data.txt"))
	if string(data) != "$Format:%H$\n" {
		t.Fatal("archive substitution altered committed input", string(data))
	}
}
