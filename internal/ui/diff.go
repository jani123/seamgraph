package ui

import (
	"fmt"
	"strings"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"seamgraph/internal/application"
)

type diffLine struct {
	Old, New         int
	Kind, Mark, Text string
}
type diffView struct {
	Rows                       []diffLine
	Before, After, Unavailable string
	Unchanged                  bool
}

func sourceDiff(before, after application.ReviewContent, changes bool) diffView {
	view := diffView{Before: sourcePlain(before.Text), After: sourcePlain(after.Text)}
	for _, content := range []application.ReviewContent{before, after} {
		if content.Unavailable != "" && content.Unavailable != "File absent at this revision" {
			view.Unavailable = content.Unavailable
			return view
		}
	}
	if len(view.Before)+len(view.After) > 250000 || strings.Count(view.Before, "\n")+strings.Count(view.After, "\n") > 10000 || diffWork(view.Before, view.After) > 2000000 {
		view.Unavailable = "File exceeds the inline diff limit. Turn off comparison to read the source."
		return view
	}
	edits := myers.ComputeEdits(span.URIFromPath("source"), view.Before, view.After)
	hunks := gotextdiff.ToUnified("base", "head", view.Before, edits).Hunks
	view.Unchanged = len(hunks) == 0
	oldLines := diffSourceLines(view.Before)
	old, newLine := 1, 1
	for _, hunk := range hunks {
		if !changes {
			appendEqual(&view, oldLines, old, hunk.FromLine, newLine)
		}
		old, newLine = hunk.FromLine, hunk.ToLine
		if changes {
			view.Rows = append(view.Rows, diffLine{Kind: "hunk", Text: fmt.Sprintf("@@ base line %d · head line %d @@", old, newLine)})
		}
		for _, line := range hunk.Lines {
			row := diffLine{Text: strings.TrimSuffix(line.Content, "\n"), Kind: "equal", Mark: " "}
			switch line.Kind {
			case gotextdiff.Delete:
				row.Old = old
				old++
				row.Kind = "removed"
				row.Mark = "−"
			case gotextdiff.Insert:
				row.New = newLine
				newLine++
				row.Kind = "added"
				row.Mark = "+"
			default:
				row.Old = old
				row.New = newLine
				old++
				newLine++
			}
			appendDiffRow(&view, row, line.Content)
		}
	}
	if !changes {
		appendEqual(&view, oldLines, old, len(oldLines)+1, newLine)
	}
	return view
}

func appendDiffRow(view *diffView, row diffLine, content string) {
	view.Rows = append(view.Rows, row)
	if row.Kind != "equal" && !strings.HasSuffix(content, "\n") {
		view.Rows = append(view.Rows, diffLine{Kind: "meta", Text: "No newline at end of file"})
	}
}
func diffSourceLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
func appendEqual(view *diffView, lines []string, from, to, newLine int) {
	for old := from; old < to && old <= len(lines); old++ {
		view.Rows = append(view.Rows, diffLine{Old: old, New: newLine, Kind: "equal", Mark: " ", Text: strings.TrimSuffix(lines[old-1], "\n")})
		newLine++
	}
}
func diffNumber(number int) string {
	if number == 0 {
		return ""
	}
	return fmt.Sprint(number)
}
func diffSide(row diffLine) string {
	if row.Kind == "removed" {
		return "before"
	}
	return "after"
}
func diffSourceLine(row diffLine) string {
	if row.Kind == "removed" {
		return diffNumber(row.Old)
	}
	return diffNumber(row.New)
}

// Bound Myers' trace allocation by the input length and the maximum number of
// edits after removing the equal prefix/suffix. This also bounds replaced files.
func diffWork(before, after string) int {
	left, right := diffSourceLines(before), diffSourceLines(after)
	total := len(left) + len(right)
	first := 0
	for first < len(left) && first < len(right) && left[first] == right[first] {
		first++
	}
	a, b := len(left), len(right)
	for a > first && b > first && left[a-1] == right[b-1] {
		a--
		b--
	}
	return total * (a + b - 2*first)
}
