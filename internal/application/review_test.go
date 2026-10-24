package application

import (
	"context"
	"seamgraph/internal/ir"
	"testing"
)

func TestReviewActivityDistinguishesRevertedChurnAndNet(t *testing.T) {
	activity := ir.Activity{Commits: []ir.ReviewCommit{
		{Files: map[string]ir.LineStats{"checkout.ts": {Added: 1, Deleted: 1}, "logo.bin": {Binary: true}}},
		{Files: map[string]ir.LineStats{"checkout.ts": {Added: 1, Deleted: 1}}},
		{Files: map[string]ir.LineStats{"checkout.ts": {Added: 1, Deleted: 1}}},
	}, Net: map[string]ir.LineStats{"logo.bin": {Binary: true}}}
	hotspots := reviewHotspots(activity)
	if len(hotspots) != 2 || hotspots[0].Path != "checkout.ts" || hotspots[0].Touches != 3 || hotspots[0].Churn != 6 || hotspots[0].Net.Added != 0 || hotspots[0].Net.Deleted != 0 {
		t.Fatal(hotspots)
	}
	if !hotspots[1].Binary || !hotspots[1].Net.Binary {
		t.Fatal("binary uncertainty lost")
	}
}
func TestReviewMarksFollowRefsAndRetainOriginalAnchors(t *testing.T) {
	file := ReviewFile{Path: "orders/order.go", BeforeDigest: "first", AfterDigest: "second"}
	entry := ReviewEntry{BaseRef: "main", HeadRef: "review", File: file.Path, Digest: reviewDigest(file), Head: "old-sha", Note: "check stock release", Viewed: true}
	r := ReviewWorkspace{BaseRef: "main", HeadRef: "review", Files: []ReviewFile{file}}
	if marks := reviewMarks(r, []ReviewEntry{entry}); len(marks) != 1 || marks[0].Status != "current" {
		t.Fatal(marks)
	}
	r.Files[0].AfterDigest = "third"
	marks := reviewMarks(r, []ReviewEntry{entry})
	if marks[0].Status != "needs reconsideration" || marks[0].Entry != entry {
		t.Fatal(marks)
	}
	r.Files = nil
	if marks := reviewMarks(r, []ReviewEntry{entry}); marks[0].Status != "anchor unavailable" {
		t.Fatal(marks)
	}
	r.HeadRef = "different"
	if marks := reviewMarks(r, []ReviewEntry{entry}); len(marks) != 0 {
		t.Fatal("mixed comparison state", marks)
	}
}

func TestReviewRejectsRecapturedWorkingChanges(t *testing.T) {
	cleaned := false
	s := Service{ports: Ports{Extract: func(_ context.Context, _, ref string) (string, string, func(), error) {
		if ref != "WORKTREE" {
			t.Fatalf("unexpected ref %s", ref)
		}
		return "snapshot", "WORKTREE:new", func() { cleaned = true }, nil
	}}}
	if _, _, err := s.reviewTree(context.Background(), "WORKTREE:old"); err == nil || !cleaned {
		t.Fatal("changed working snapshot accepted or leaked", err)
	}
}
