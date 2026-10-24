package application

import (
	"strings"
	"testing"
)

func TestCommentAnchorsValidateBothSides(t *testing.T) {
	code := CodeBrowser{Before: ReviewContent{Text: "old\nremoved\n"}, After: ReviewContent{Text: "new\n"}}
	for _, anchor := range []struct {
		side string
		line int
	}{{"bad", 1}, {"after", 0}, {"after", 2}, {"before", 3}, {"", 1}, {"before", -1}} {
		if _, err := commentContext(code, anchor.side, anchor.line); err == nil {
			t.Fatalf("accepted invalid anchor %+v", anchor)
		}
	}
	context, err := commentContext(code, "before", 2)
	if err != nil || !strings.Contains(context, "removed") {
		t.Fatal(context, err)
	}
	code.After = ReviewContent{Unavailable: "File absent at this revision"}
	if _, err := commentContext(code, "after", 1); err == nil {
		t.Fatal("accepted absent head line")
	}
}

func TestCommentsKeepMultipleAnchorsAndFileReview(t *testing.T) {
	file := ReviewFile{Path: "file.go", BeforeDigest: "a", AfterDigest: "b"}
	base := ReviewEntry{BaseRef: "main", HeadRef: "feature", File: file.Path, Digest: reviewDigest(file), Note: "file note"}
	first, second := base, base
	first.Comment, first.CommentID, first.Side, first.Line = true, "one", "before", 1
	second.Comment, second.CommentID, second.Side, second.Line = true, "two", "after", 2
	r := ReviewWorkspace{BaseRef: "main", HeadRef: "feature", Files: []ReviewFile{file}}
	marks := reviewMarks(r, []ReviewEntry{base, first, second})
	if len(marks) != 3 {
		t.Fatal(marks)
	}
	r.Files[0].AfterDigest = "changed"
	for _, mark := range reviewMarks(r, []ReviewEntry{base, first, second}) {
		if mark.Status != "needs reconsideration" {
			t.Fatal(mark)
		}
	}
}

func TestCommentsMarkdownContainsAllAnchorsAndSafeFences(t *testing.T) {
	code := CodeBrowser{Base: "base-sha", Revision: "head-sha", Selected: "only-this.txt", Marks: []ReviewMark{
		{Status: "needs reconsideration", Entry: ReviewEntry{File: "other`file.go", Base: "old-base", Head: "old-head", Side: "before", Line: 7, Note: "Check **rollback** <script>bad()</script>", Context: "```\nold source\n```"}},
		{Status: "current", Entry: ReviewEntry{File: "second.go", Side: "after", Line: 9, Note: "Second finding"}},
	}}
	markdown := CommentsMarkdown(code, "main", "feature")
	for _, text := range []string{"other`file.go", "Base line 7", "Head line 9", "needs reconsideration", "old-head", "Second finding", "**rollback**", "&lt;script&gt;", "````text\n```\nold source"} {
		if !strings.Contains(markdown, text) {
			t.Errorf("missing %q in %s", text, markdown)
		}
	}
	if strings.Contains(markdown, "<script>") {
		t.Fatal("unescaped HTML in comment text")
	}
	if !strings.Contains(CommentsMarkdown(CodeBrowser{}, "a", "b"), "No comments recorded") {
		t.Fatal("missing empty export state")
	}
}

func TestLongContextKeepsTheCommentedLine(t *testing.T) {
	code := CodeBrowser{After: ReviewContent{Text: strings.Repeat("preceding long line", 200) + "\nimportant anchor\nfollowing\n"}}
	context, err := commentContext(code, "after", 2)
	if err != nil || !strings.Contains(context, "important anchor") {
		t.Fatal(context, err)
	}
	if len([]rune(context)) > 1200 {
		t.Fatal("unbounded context")
	}
}
