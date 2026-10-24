package bootstrap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"seamgraph/internal/application"
)

func TestCommentsPersistWithoutArchitectureAndRejectMovedComparison(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %s: %v", out, err)
		}
	}
	file := filepath.Join(root, "change.txt")
	if err := os.WriteFile(file, []byte("old value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("init", "-q")
	git("add", ".")
	git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "test: base")
	if err := os.WriteFile(file, []byte("new value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	code, err := service.Code(ctx, "WORKTREE", "HEAD", "change.txt", "")
	if err != nil {
		t.Fatal(err)
	}
	entry := application.ReviewEntry{Comment: true, BaseRef: "HEAD", HeadRef: "WORKTREE", Base: code.Base, Head: code.Revision, File: "change.txt", Side: "after", Line: 1, Note: "Check the new value"}
	if err := service.RecordReview(ctx, entry); err != nil {
		t.Fatal(err)
	}
	entry.Side, entry.Note = "before", "Keep the old fallback"
	if err := service.RecordReview(ctx, entry); err != nil {
		t.Fatal(err)
	}
	service, err = New(root)
	if err != nil {
		t.Fatal(err)
	}
	code, err = service.Code(ctx, "WORKTREE", "HEAD", "", "")
	if err != nil || len(code.Marks) != 2 {
		t.Fatal(code.Marks, err)
	}
	if code.Marks[0].Entry.CommentID == code.Marks[1].Entry.CommentID {
		t.Fatal("comments overwrote each other")
	}
	if err := os.WriteFile(file, []byte("next value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordReview(ctx, entry); err == nil {
		t.Fatal("accepted a stale comparison")
	}
	code, err = service.Code(ctx, "WORKTREE", "HEAD", "", "")
	if err != nil {
		t.Fatal(err)
	}
	markdown := application.CommentsMarkdown(code, "HEAD", "WORKTREE")
	for _, text := range []string{"needs reconsideration", "old value", "new value", "Check the new value", "Keep the old fallback"} {
		if !strings.Contains(markdown, text) {
			t.Fatalf("missing %q in %s", text, markdown)
		}
	}
}
