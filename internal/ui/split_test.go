package ui

import (
	"seamgraph/internal/application"
	"testing"
)

func TestSplitDiffAlignsReplacementAndPreservesLineAnchors(t *testing.T) {
	view := sourceDiff(application.ReviewContent{Text: "same\nold\nextra\n"}, application.ReviewContent{Text: "same\nnew\n"}, false)
	rows := splitRows(view)
	if len(rows) != 4 || rows[1].Before.Old != 2 || rows[1].After.New != 2 || rows[1].Before.Text != "old" || rows[1].After.Text != "new" || rows[2].After.New != 0 {
		t.Fatal(rows)
	}
}
