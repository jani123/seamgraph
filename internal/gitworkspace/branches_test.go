package gitworkspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBranchChoicesUseLocalRefsWithoutCheckoutOrFetch(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		b, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(string(b), err)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("-c", "user.name=Test", "-c", "user.email=test@example.test", "commit", "-qm", "test: base")
	run("switch", "-qc", "feature/payments")
	run("update-ref", "refs/remotes/origin/main", "HEAD")
	run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	choices, err := Branches(context.Background(), root)
	if err != nil || choices.Current != "feature/payments" || choices.Default != "main" || len(choices.Items) != 3 {
		t.Fatal(choices, err)
	}
	current, remote := false, false
	for _, item := range choices.Items {
		if item.Ref == "feature/payments" {
			current = item.Current
		}
		if item.Ref == "refs/remotes/origin/main" {
			remote = item.Remote && item.Name == "origin/main"
		}
	}
	if !current || !remote {
		t.Fatal(choices)
	}
	run("branch", "develop")
	run("update-ref", "refs/remotes/origin/develop", "HEAD")
	run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/develop")
	defaultChoices, err := Branches(context.Background(), root)
	if err != nil || defaultChoices.Default != "develop" {
		t.Fatal("cached remote default was ignored", defaultChoices, err)
	}
	run("checkout", "--detach", "-q")
	detached, err := Branches(context.Background(), root)
	if err != nil || detached.Current != "" || len(detached.Items) != 5 {
		t.Fatal(detached, err)
	}
	if _, err := Branches(context.Background(), t.TempDir()); err == nil {
		t.Fatal("non-Git directory claimed branch choices")
	}
}
