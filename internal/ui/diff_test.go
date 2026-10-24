package ui

import (
	"seamgraph/internal/application"
	"strings"
	"testing"
)

func numberedSource(text string) application.ReviewContent {
	var out strings.Builder
	for i, line := range strings.Split(text, "\n") {
		out.WriteString(diffNumber(i+1) + "  " + line + "\n")
	}
	return application.ReviewContent{Text: out.String()}
}
func TestUnifiedDiffKeepsLineCoordinatesAndMarks(t *testing.T) {
	before, after := numberedSource("one\ntwo\nthree\n"), numberedSource("one\nchanged\nthree\n")
	view := sourceDiff(before, after, false)
	if view.Unavailable != "" || len(view.Rows) != 4 {
		t.Fatal(view)
	}
	removed, added := view.Rows[1], view.Rows[2]
	if removed.Kind != "removed" || removed.Old != 2 || removed.New != 0 || removed.Text != "two" {
		t.Fatal(removed)
	}
	if added.Kind != "added" || added.New != 2 || added.Old != 0 || added.Text != "changed" {
		t.Fatal(added)
	}
	if view.Rows[3].Old != 3 || view.Rows[3].New != 3 {
		t.Fatal(view.Rows[3])
	}
}
func TestUnifiedDiffAddedRemovedUnchangedAndLimits(t *testing.T) {
	absent := application.ReviewContent{Unavailable: "File absent at this revision"}
	for _, removed := range []bool{false, true} {
		before, after := absent, numberedSource("new\n")
		want := "added"
		if removed {
			before, after = after, before
			want = "removed"
		}
		view := sourceDiff(before, after, true)
		if view.Unavailable != "" || len(view.Rows) != 2 || view.Rows[1].Kind != want {
			t.Fatal(view)
		}
	}
	same := sourceDiff(numberedSource("same\n"), numberedSource("same\n"), true)
	if !same.Unchanged || len(same.Rows) != 0 {
		t.Fatal(same)
	}
	binary := sourceDiff(application.ReviewContent{Unavailable: "Binary file; text diff unavailable"}, absent, true)
	if binary.Unavailable == "" {
		t.Fatal(binary)
	}
	large := sourceDiff(numberedSource(strings.Repeat("old\n", 1500)), numberedSource(strings.Repeat("new\n", 1500)), true)
	if large.Unavailable == "" {
		t.Fatal("unbounded replaced file")
	}
}
func TestChangesOnlyRetainsDirectoryContext(t *testing.T) {
	browser := application.CodeBrowser{Files: []application.ReviewFile{
		{Path: "src/unchanged.go", Status: "unchanged"}, {Path: "src/changed.go", Status: "modified"}, {Path: "removed.go", Status: "removed"},
	}}
	tree := codeTree(browser, CodeFilter{ChangesOnly: true})
	if len(tree.Files) != 2 || len(tree.Files[0].Children) != 1 {
		t.Fatal(tree.Files)
	}
	if tree.Files[0].Children[0].Name != "changed.go" {
		t.Fatal(tree.Files)
	}
}

func TestUnifiedDiffExplainsNewlineOnlyChanges(t *testing.T) {
	view := sourceDiff(numberedSource("same\n"), numberedSource("same"), true)
	found := false
	for _, row := range view.Rows {
		if row.Kind == "meta" && row.Text == "No newline at end of file" {
			found = true
		}
	}
	if !found {
		t.Fatal(view)
	}
}
