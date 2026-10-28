package application

import (
	"context"
	"crypto/rand"
	"fmt"
	"html"
	"strings"
	"time"

	"seamgraph/internal/ir"
)

func (s *Service) codeComments(code CodeBrowser, base, head string) (CodeBrowser, error) {
	if base == "" || s.ports.ReadReview == nil {
		return code, nil
	}
	entries, err := s.ports.ReadReview(s.root)
	if err != nil {
		return code, err
	}
	code.Marks = reviewMarks(ReviewWorkspace{BaseRef: base, HeadRef: head, Files: code.Files, Comparison: Result{Base: code.Base, Head: code.Revision}}, entries)
	return code, nil
}

func (s *Service) recordComment(ctx context.Context, entry ReviewEntry) error {
	if !ir.ValidPath(entry.File) || (!entry.Deleted && (strings.TrimSpace(entry.Note) == "" || len(entry.Note) > 8000)) {
		return fmt.Errorf("a comment needs a valid file and 1–8000 bytes of text")
	}
	if entry.Deleted && entry.CommentID == "" {
		return fmt.Errorf("select an existing comment to delete")
	}
	file := entry.File
	if entry.CommentID != "" {
		file = ""
	}
	code, err := s.Code(ctx, entry.HeadRef, entry.BaseRef, file, "")
	if err != nil {
		return err
	}
	if entry.Base == "" || entry.Base != code.Base || entry.Head != code.Revision {
		return fmt.Errorf("comparison changed; refresh before saving")
	}
	if entry.CommentID != "" {
		return s.editComment(code, entry)
	}
	excerpt, err := commentContext(code, entry.Side, entry.Line)
	if err != nil {
		return err
	}
	for _, file := range code.Files {
		if file.Path == entry.File {
			entry.Digest = reviewDigest(file)
		}
	}
	entry.Context = excerpt
	entry.CommentID = rand.Text()
	entry.RecordedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return s.ports.AppendReview(s.root, entry)
}

func commentContext(code CodeBrowser, side string, line int) (string, error) {
	content := code.After
	switch side {
	case "before":
		content = code.Before
	case "after":
	case "":
		if line != 0 {
			return "", fmt.Errorf("a line comment needs a base or head side")
		}
		if content.Text == "" {
			content = code.Before
		}
	default:
		return "", fmt.Errorf("invalid comment side")
	}
	if side != "" {
		lines := strings.Split(strings.TrimSuffix(content.Text, "\n"), "\n")
		if content.Unavailable != "" || content.Text == "" || line < 1 || line > len(lines) {
			return "", fmt.Errorf("comment line is outside the selected source")
		}
		content.Text = strings.Join(lines[max(0, line-4):min(len(lines), line+3)], "\n")
		if len([]rune(content.Text)) > 1200 {
			content.Text = lines[line-1]
		}
	}
	runes := []rune(content.Text)
	return string(runes[:min(1200, len(runes))]), nil
}

// CommentsMarkdown collects the entire comparison's comments, regardless of the
// selected file or UI filters. Original anchors remain explicit after edits.
func CommentsMarkdown(code CodeBrowser, baseRef, headRef string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# Review comments\n\nBase: %s (%s)\n\nHead: %s (%s)\n\n", markdownLiteral(baseRef), markdownLiteral(code.Base), markdownLiteral(headRef), markdownLiteral(code.Revision))
	count := 0
	for _, mark := range code.Marks {
		if strings.TrimSpace(mark.Entry.Note) == "" {
			continue
		}
		count++
		writeCommentMarkdown(&out, mark, count)
	}
	if count == 0 {
		out.WriteString("No comments recorded for this comparison.\n")
	}
	return out.String()
}

func writeCommentMarkdown(out *strings.Builder, mark ReviewMark, number int) {
	entry := mark.Entry
	fmt.Fprintf(out, "## Comment %d\n\nFile: %s\n\n", number, markdownLiteral(entry.File))
	if entry.Line > 0 {
		fmt.Fprintf(out, "Anchor: %s line %d\n\n", commentSide(entry.Side), entry.Line)
	}
	fmt.Fprintf(out, "Status: %s · Resolved: %t\n\nOriginal comparison: %s → %s\n\n%s\n\n", mark.Status, entry.Resolved, markdownLiteral(entry.Base), markdownLiteral(entry.Head), html.EscapeString(entry.Note))
	if entry.Context != "" {
		fence := "```"
		for strings.Contains(entry.Context, fence) {
			fence += "`"
		}
		fmt.Fprintf(out, "Original source context:\n\n%stext\n%s\n%s\n\n", fence, entry.Context, fence)
	}
}
func markdownLiteral(text string) string {
	fence := "`"
	for strings.Contains(text, fence) {
		fence += "`"
	}
	return fence + " " + strings.ReplaceAll(strings.ReplaceAll(text, "\r", " "), "\n", " ") + " " + fence
}
func commentSide(side string) string {
	if side == "before" {
		return "Base"
	}
	return "Head"
}

// Edits append a new version of the same comment, preserving its original
// revision, source context and digest even when that anchor has become stale.
func (s *Service) editComment(code CodeBrowser, update ReviewEntry) error {
	for _, mark := range code.Marks {
		original := mark.Entry
		if !original.Comment || original.CommentID != update.CommentID || original.File != update.File {
			continue
		}
		version := original.UpdatedAt
		if version == "" {
			version = original.RecordedAt
		}
		if update.EditVersion == "" || update.EditVersion != version {
			return fmt.Errorf("comment changed; reopen it before saving")
		}
		original.EditVersion = update.EditVersion
		original.Deleted = update.Deleted
		if !update.Deleted {
			original.Note = update.Note
		}
		original.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		return s.ports.AppendReview(s.root, original)
	}
	return fmt.Errorf("comment no longer exists in this comparison")
}
