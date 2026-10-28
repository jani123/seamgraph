package application

import (
	"seamgraph/internal/ir"
	"testing"
)

func TestActivityFiltersKeepDistinctCommitsAndFullRangeTotals(t *testing.T) {
	r := ReviewWorkspace{Options: map[string]string{"group": "directory", "metric": "churn", "exclude": "generated,vendor"}, TypeFilter: "fix", Activity: ir.Activity{Commits: []ir.ReviewCommit{
		{Type: "fix", Scope: "api", Files: map[string]ir.LineStats{"src/a.go": {Added: 2}, "src/b.go": {Deleted: 1}, "vendor/x.go": {Added: 8}}},
		{Type: "revert", Files: map[string]ir.LineStats{"src/a.go": {Deleted: 2}}},
	}, Net: map[string]ir.LineStats{"src/b.go": {Deleted: 1}}}}
	r.Hotspots = reviewHotspots(r.Activity)
	view := SummarizeReview(r)
	if len(view.Commits) != 1 || len(view.Hotspots) != 1 {
		t.Fatal(view)
	}
	row := view.Hotspots[0]
	if row.Path != "src" || row.Touches != 2 || row.Churn != 5 || row.Net.Deleted != 1 {
		t.Fatal(row)
	}
	r.Options["commitScope"] = "other"
	if view = SummarizeReview(r); len(view.Commits) != 0 || len(view.Hotspots) != 0 {
		t.Fatal(view)
	}
}
