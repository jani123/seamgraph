package gitworkspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchRangesKeepTreeAndHistoryIdentity(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.test", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.test")
		b, err := command.CombinedOutput()
		if err != nil {
			t.Fatal(string(b), err)
		}
		return strings.TrimSpace(string(b))
	}
	run("init", "-q")
	tree := run("mktree")
	commit := func(message string, parents ...string) string {
		args := []string{"commit-tree", tree, "-m", message}
		for _, parent := range parents {
			args = append(args, "-p", parent)
		}
		return run(args...)
	}
	base := commit("base")
	target := commit("target", base)
	review := commit("review", base)
	run("update-ref", "refs/heads/target", target)
	run("update-ref", "refs/heads/review", review)
	status := run("status", "--porcelain")
	r, err := BranchRange(context.Background(), root, "target", "review", "merge-base")
	if err != nil || r.Base != base || r.Head != review || r.TargetRevision != target {
		t.Fatal(r, err)
	}
	r, err = BranchRange(context.Background(), root, "target", "review", "target-tip")
	if err != nil || r.Base != target || r.Head != review {
		t.Fatal(r, err)
	}
	a := commit("left merge", target, review)
	b := commit("right merge", review, target)
	if _, err = BranchRange(context.Background(), root, a, b, "merge-base"); err == nil || !strings.Contains(err.Error(), "ambiguous merge bases") {
		t.Fatal(err)
	}
	unrelated := commit("unrelated")
	if _, err = BranchRange(context.Background(), root, base, unrelated, "merge-base"); err == nil || !strings.Contains(err.Error(), "no merge base") {
		t.Fatal(err)
	}
	if _, err = BranchRange(context.Background(), root, "missing", review, "merge-base"); err == nil || !strings.Contains(err.Error(), "target revision") {
		t.Fatal(err)
	}
	if run("status", "--porcelain") != status {
		t.Fatal("review changed checkout or index")
	}
	// A shallow boundary must never silently produce partial history totals.
	if err = os.WriteFile(filepath.Join(root, ".git", "shallow"), []byte(base+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = BranchRange(context.Background(), root, target, review, "merge-base"); err == nil || !strings.Contains(err.Error(), "shallow") {
		t.Fatal(err)
	}
}
