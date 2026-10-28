package bootstrap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"seamgraph/internal/application"
	"strings"
	"testing"
)

func TestUnconfiguredBranchReviewKeepsMovesChecklistAndIdentity(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.test", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.test")
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(string(b), err)
		}
		return strings.TrimSpace(string(b))
	}
	write := func(file, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, file), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write("old.py", "def read(): return 1\n")
	git("add", ".")
	git("commit", "-qm", "feat: base")
	base := git("rev-parse", "HEAD")
	git("branch", "target")
	git("switch", "-qc", "review")
	git("mv", "old.py", "new.py")
	git("commit", "-qm", "refactor: move")
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	r, err := service.Review(ctx, "merge-base:target", "review", "new.py", "")
	if err != nil || r.Range.Base != base || r.ArchitectureIssue == "" {
		t.Fatal(r, err)
	}
	if r.Code.Before.Text == "" || r.Code.Before.Text != r.Code.After.Text {
		t.Fatal("exact rename lost before source", r.Code)
	}
	entry := application.ReviewEntry{Checklist: true, BaseRef: r.BaseRef, HeadRef: r.HeadRef, Base: r.Comparison.Base, Head: r.Comparison.Head, Note: "- [x] Read the move"}
	if err = service.RecordReview(ctx, entry); err != nil {
		t.Fatal(err)
	}
	service, err = New(root)
	if err != nil {
		t.Fatal(err)
	}
	r, err = service.Review(ctx, entry.BaseRef, entry.HeadRef, "new.py", "")
	if err != nil || len(r.Marks) != 1 || r.Marks[0].Status != "current" {
		t.Fatal(r.Marks, err)
	}
	write("new.py", "def read(): return 2\n")
	git("commit", "-qam", "fix: body")
	if err = service.RecordReview(ctx, entry); err == nil {
		t.Fatal("accepted stale checklist")
	}
	r, err = service.Review(ctx, entry.BaseRef, entry.HeadRef, "new.py", "")
	if err != nil || r.Marks[0].Status != "needs reconsideration" {
		t.Fatal(r.Marks, err)
	}
	other, err := service.Review(ctx, "target-tip:target", "review", "new.py", "")
	if err != nil || len(other.Marks) != 0 {
		t.Fatal("comparison modes shared review state", other.Marks, err)
	}
}
