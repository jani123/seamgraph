package projectfiles

import (
	"os"
	"path/filepath"
	"testing"

	"seamgraph/internal/application"
)

func TestReviewLogPreservesAnchorsAndRejectsUnsafeStores(t *testing.T) {
	root := t.TempDir()
	if entries, err := ReadReview(root); err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
	first := application.ReviewEntry{File: "orders/order.go", Head: "old", Note: "original context", Context: "first source"}
	second := first
	second.Head = "new"
	second.Note = "acknowledged"
	second.Viewed = true
	for _, entry := range []application.ReviewEntry{first, second} {
		if err := AppendReview(root, entry); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := ReadReview(root)
	if err != nil || len(entries) != 2 || entries[0] != first || entries[1] != second {
		t.Fatal(entries, err)
	}
	lock := filepath.Join(root, ".seamgraph/reviews.lock")
	if err := os.WriteFile(lock, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := AppendReview(root, second); err == nil {
		t.Fatal("concurrent writer accepted")
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".seamgraph/reviews.jsonl")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../notes", path); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadReview(root); err == nil {
		t.Fatal("symlink review log accepted")
	}
	if err := AppendReview(root, second); err == nil {
		t.Fatal("symlink review log written")
	}
	content, err := os.ReadFile(filepath.Join(root, "notes"))
	if err != nil || string(content) != "secret" {
		t.Fatal(string(content), err)
	}
}

func TestCommentAppendRechecksTheVersionUnderLock(t *testing.T) {
	root := t.TempDir()
	first := application.ReviewEntry{Comment: true, CommentID: "one", File: "file.go", BaseRef: "main", HeadRef: "review", Note: "first", RecordedAt: "original"}
	if err := AppendReview(root, first); err != nil {
		t.Fatal(err)
	}
	updated := first
	updated.Note = "updated"
	updated.UpdatedAt = "newer"
	updated.EditVersion = first.RecordedAt
	if err := AppendReview(root, updated); err != nil {
		t.Fatal(err)
	}
	stale := updated
	stale.Note = "lost update"
	if err := AppendReview(root, stale); err == nil {
		t.Fatal("stale edit overwrote newer comment")
	}
	entries, err := ReadReview(root)
	if err != nil || len(entries) != 2 || entries[1].Note != "updated" || entries[1].Context != first.Context || entries[1].EditVersion != "" {
		t.Fatal(entries, err)
	}
	removed := updated
	removed.Deleted = true
	removed.EditVersion = updated.UpdatedAt
	removed.UpdatedAt = "deleted"
	if err := AppendReview(root, removed); err != nil {
		t.Fatal(err)
	}
	revive := removed
	revive.Deleted = false
	revive.EditVersion = removed.UpdatedAt
	if err := AppendReview(root, revive); err == nil {
		t.Fatal("deleted identity accepted a new edit")
	}

}
