package gitworkspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFingerprintTracksInputsAndContext(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.go")
	if err := os.WriteFile(source, []byte("package app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, before, err := Identity(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(root, ".seamgraph"), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, ".seamgraph/evidence.json"), []byte("generated artifact"), 0644); err != nil {
		t.Fatal(err)
	}
	_, after, err := Identity(context.Background(), root)
	if err != nil || before != after {
		t.Fatal("evidence invalidated itself", err)
	}
	t.Setenv("GOFLAGS", "-tags=selected-context")
	_, contextHash, err := Identity(context.Background(), root)
	if err != nil || contextHash == after {
		t.Fatal("build context not fingerprinted", err)
	}
	if err = os.WriteFile(source, []byte("package app\n// source change\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, changed, err := Identity(context.Background(), root)
	if err != nil || changed == contextHash {
		t.Fatal("implementation change not fingerprinted", err)
	}
}
func TestWorktreeGitMetadataIsExcluded(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "data"), []byte("input"), 0644); err != nil {
		t.Fatal(err)
	}
	_, before, err := Identity(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /unavailable/worktree-metadata\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, after, err := Identity(context.Background(), root)
	if err != nil || after != before {
		t.Fatal("worktree metadata changed source fingerprint", err)
	}
}
