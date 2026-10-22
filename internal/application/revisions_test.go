package application_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "seamgraph/internal/application"
	registry "seamgraph/internal/bootstrap"
	git "seamgraph/internal/gitworkspace"
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
		svc, err := registry.New(root)
		if err != nil {
			return ir.Snapshot{}, policy.Policy{}, err
		}
		session, err := svc.InspectProject(ctx, root, "")
		if err != nil {
			return ir.Snapshot{}, policy.Policy{}, err
		}
		defer session.Close()
		return session.Snapshot, session.Policy, nil
	}
	r, err := Compare(context.Background(), root, "HEAD~1", "HEAD", git.Extract, analyze)
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
	if _, err = git.Revision(context.Background(), root, "--help"); err == nil {
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
	digest := "first-config"
	cache := &Cache{Context: "first", ConfigKey: func(string) (string, error) { return digest, nil }}
	for i := 0; i < 2; i++ {
		if _, err := cache.Compare(context.Background(), root, "HEAD", "HEAD", git.Extract, analyze); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatal("identical immutable snapshots not cached", calls)
	}
	digest = "changed-config"
	if _, err := cache.Compare(context.Background(), root, "HEAD", "HEAD", git.Extract, analyze); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("changed configuration reused cache", calls)
	}
	cache.Context = "changed"
	if _, err := cache.Compare(context.Background(), root, "HEAD", "HEAD", git.Extract, analyze); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
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
	dir, _, cleanup, err := git.Extract(context.Background(), root, "HEAD")
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

func TestWorkingComparisonIncludesInternalChangesAndRefreshesCache(t *testing.T) {
	root := t.TempDir()
	err := filepath.WalkDir("../../testdata/projects/go", func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("../../testdata/projects/go", p)
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
	command(t, root, "add", ".")
	command(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	svc, err := registry.New(root)
	if err != nil {
		t.Fatal(err)
	}
	world := filepath.Join(root, "world/world.go")
	original, err := os.ReadFile(world)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(world, append(original, []byte("\n// internal edit\n")...), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Compare(context.Background(), "HEAD", "WORKTREE")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diff.Changes) != 0 || len(result.Modules) != 1 || result.Modules[0].ID != "module:world" || len(result.Modules[0].Files) != 1 || len(result.Modules[0].Seams) != 0 {
		t.Fatalf("internal-only edit: %+v", result)
	}
	committed, err := svc.Compare(context.Background(), "HEAD", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(committed.Files) != 0 || len(committed.Modules) != 0 {
		t.Fatal("working bytes leaked into committed comparison")
	}
	if err = os.WriteFile(filepath.Join(root, "npc/internal.go"), []byte("package npc\nvar privateValue = 3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	command(t, root, "add", "npc/internal.go")
	if err = os.WriteFile(filepath.Join(root, "world/untracked.go"), []byte("package world\nvar otherPrivateValue = 4\n"), 0644); err != nil {
		t.Fatal(err)
	}
	next, err := svc.Compare(context.Background(), "HEAD", "WORKTREE")
	if err != nil {
		t.Fatal(err)
	}
	if next.Head == result.Head || len(next.Files) != 3 || len(next.Modules) != 2 {
		t.Fatalf("stale or incomplete working comparison: %+v", next)
	}
	// Isolate a policy document comment: config badge, no source badge.
	os.WriteFile(world, original, 0644)
	os.Remove(filepath.Join(root, "npc/internal.go"))
	os.Remove(filepath.Join(root, "world/untracked.go"))
	configFile := filepath.Join(root, "seamgraph.yaml")
	data, _ := os.ReadFile(configFile)
	os.WriteFile(configFile, append(data, []byte("\n# policy note\n")...), 0644)
	configOnly, err := svc.Compare(context.Background(), "HEAD", "WORKTREE")
	if err != nil {
		t.Fatal(err)
	}
	if !configOnly.ConfigurationChanged || len(configOnly.Modules) != 0 {
		t.Fatalf("configuration claimed source changes: %+v", configOnly)
	}
}
