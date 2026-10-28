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
	original := code.Marks[0].Entry
	update := original
	update.Base, update.Head = code.Base, code.Revision
	update.EditVersion = original.RecordedAt
	update.Note = "Edited finding"
	if err := service.RecordReview(ctx, update); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordReview(ctx, update); err == nil {
		t.Fatal("stale comment edit accepted")
	}
	edited, err := service.Code(ctx, "WORKTREE", "HEAD", "", "")
	if err != nil || len(edited.Marks) != 2 {
		t.Fatal(edited.Marks, err)
	}
	found := false
	for _, mark := range edited.Marks {
		if mark.Entry.CommentID != original.CommentID {
			continue
		}
		found = true
		if mark.Entry.Note != "Edited finding" || mark.Status != "needs reconsideration" || mark.Entry.Context != original.Context || mark.Entry.Head != original.Head || mark.Entry.Digest != original.Digest || mark.Entry.UpdatedAt == "" {
			t.Fatal("edit lost original anchor", mark)
		}
	}
	if !found {
		t.Fatal("edit created a new identity")
	}
	for _, mark := range edited.Marks {
		if mark.Entry.CommentID != original.CommentID {
			continue
		}
		removal := mark.Entry
		removal.Base, removal.Head = edited.Base, edited.Revision
		removal.EditVersion = removal.UpdatedAt
		removal.Deleted = true
		removal.Note = ""
		if err := service.RecordReview(ctx, removal); err != nil {
			t.Fatal(err)
		}
		removal.Deleted = false
		removal.Note = "revive"
		if err := service.RecordReview(ctx, removal); err == nil {
			t.Fatal("deleted comment was revived")
		}
	}
	restarted, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	remaining, err := restarted.Code(ctx, "WORKTREE", "HEAD", "", "")
	if err != nil || len(remaining.Marks) != 1 || remaining.Marks[0].Entry.CommentID == original.CommentID {
		t.Fatal(remaining.Marks, err)
	}

	markdown := application.CommentsMarkdown(code, "HEAD", "WORKTREE")
	for _, text := range []string{"needs reconsideration", "old value", "new value", "Check the new value", "Keep the old fallback"} {
		if !strings.Contains(markdown, text) {
			t.Fatalf("missing %q in %s", text, markdown)
		}
	}
}
