package gitworkspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoryRetainsExactParentAndOriginalMessage(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("first\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "test: initial")
	first := run("rev-parse", "HEAD")
	initial, err := activityCommit(context.Background(), root, first, "")
	if err != nil || initial.Parent != "" {
		t.Fatal(initial, err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("second\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	message := "fix: update file\n\nPreserve this explanation."
	run("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", message)
	second := run("rev-parse", "HEAD")
	commit, err := activityCommit(context.Background(), root, second, "")
	if err != nil || commit.Parent != first || commit.Message != message+"\n" {
		t.Fatal(commit, err)
	}
}
