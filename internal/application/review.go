package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"seamgraph/internal/ir"
)

type ReviewFile struct {
	Path         string `json:"path"`
	Status       string `json:"status"`
	Module       string `json:"module,omitempty"`
	BeforeDigest string `json:"beforeDigest,omitempty"`
	AfterDigest  string `json:"afterDigest,omitempty"`
}
type ReviewEntry struct {
	Comment    bool   `json:"comment,omitempty"`
	CommentID  string `json:"commentId,omitempty"`
	Side       string `json:"side,omitempty"`
	Line       int    `json:"line,omitempty"`
	BaseRef    string `json:"baseRef"`
	HeadRef    string `json:"headRef"`
	Base       string `json:"base"`
	Head       string `json:"head"`
	File       string `json:"file"`
	Digest     string `json:"digest"`
	Context    string `json:"context"`
	Note       string `json:"note"`
	Viewed     bool   `json:"viewed"`
	Resolved   bool   `json:"resolved"`
	RecordedAt string `json:"recordedAt"`
}
type ReviewMark struct {
	Entry  ReviewEntry `json:"entry"`
	Status string      `json:"status"`
}
type ReviewContent struct {
	Text        string `json:"text,omitempty"`
	Unavailable string `json:"unavailable,omitempty"`
}
type Hotspot struct {
	Path    string       `json:"path"`
	Touches int          `json:"touches"`
	Added   int          `json:"added"`
	Deleted int          `json:"deleted"`
	Churn   int          `json:"churn"`
	Binary  bool         `json:"binary"`
	Net     ir.LineStats `json:"net"`
}
type ReviewWorkspace struct {
	Code           CodeBrowser   `json:"code"`
	PathFilter     string        `json:"pathFilter"`
	TypeFilter     string        `json:"typeFilter"`
	BaseRef        string        `json:"baseRef"`
	HeadRef        string        `json:"headRef"`
	Comparison     Result        `json:"comparison"`
	Files          []ReviewFile  `json:"files"`
	Activity       ir.Activity   `json:"activity"`
	Hotspots       []Hotspot     `json:"hotspots"`
	Selected       string        `json:"selected,omitempty"`
	Before         ReviewContent `json:"before"`
	After          ReviewContent `json:"after"`
	Object         *ir.Symbol    `json:"object,omitempty"`
	References     []ir.Relation `json:"references"`
	PossibleImpact []string      `json:"possibleImpact"`
	Marks          []ReviewMark  `json:"marks"`
}

func (s *Service) Review(ctx context.Context, base, head, file, object string) (ReviewWorkspace, error) {
	r := ReviewWorkspace{BaseRef: base, HeadRef: head, Selected: file}
	comparison, err := s.Compare(ctx, base, head)
	if err != nil {
		return r, err
	}
	r.Comparison = comparison
	headCode := comparison.Head
	if strings.HasPrefix(headCode, "WORKTREE:") {
		headCode = "WORKTREE"
	}
	baseCode := comparison.Base
	if strings.HasPrefix(baseCode, "WORKTREE:") {
		baseCode = "WORKTREE"
	}
	r.Code, err = s.Code(ctx, headCode, baseCode, file, object)
	if err != nil {
		return r, err
	}
	if r.Code.Revision != comparison.Head || r.Code.Base != comparison.Base {
		return r, fmt.Errorf("working changes moved during code review; refresh")
	}
	r.Code.ReviewBaseRef = base
	r.Code.ReviewHeadRef = head
	r.Selected = r.Code.Selected
	if err := s.reviewFiles(ctx, &r); err != nil {
		return r, err
	}
	r.Activity, err = s.ports.Activity(ctx, s.root, comparison.Base, comparison.Head, r.Selected)
	if err != nil {
		return r, err
	}
	r.Hotspots = reviewHotspots(r.Activity)
	r.selectObject(object)
	entries, err := s.ports.ReadReview(s.root)
	if err != nil {
		return r, err
	}
	r.Marks = reviewMarks(r, entries)
	return r, nil
}

func (s *Service) reviewFiles(ctx context.Context, r *ReviewWorkspace) error {
	before, cleanup, err := s.reviewTree(ctx, r.Comparison.Base)
	if err != nil {
		return err
	}
	defer cleanup()
	after, cleanupHead, err := s.reviewTree(ctx, r.Comparison.Head)
	if err != nil {
		return err
	}
	defer cleanupHead()
	a, err := s.ports.Inventory(before)
	if err != nil {
		return err
	}
	b, err := s.ports.Inventory(after)
	if err != nil {
		return err
	}
	r.Files = reviewFiles(a, b, r.Comparison)
	if r.Selected == "" {
		return nil
	}
	if !slices.ContainsFunc(r.Files, func(f ReviewFile) bool { return f.Path == r.Selected }) {
		return fmt.Errorf("unknown review file")
	}
	r.Before = s.reviewContent(before, r.Selected, a[r.Selected])
	r.After = s.reviewContent(after, r.Selected, b[r.Selected])
	return nil
}

// Re-extraction must preserve the captured comparison identity on both sides.
func (s *Service) reviewTree(ctx context.Context, identity string) (string, func(), error) {
	ref := identity
	if strings.HasPrefix(ref, "WORKTREE:") {
		ref = "WORKTREE"
	}
	root, actual, cleanup, err := s.ports.Extract(ctx, s.root, ref)
	if err != nil {
		return "", cleanup, err
	}
	if actual != identity {
		cleanup()
		return "", func() {}, fmt.Errorf("working changes moved during review; refresh")
	}
	return root, cleanup, nil
}

func reviewFiles(before, after map[string]string, comparison Result) []ReviewFile {
	files := map[string]ReviewFile{}
	for name, digest := range before {
		files[name] = ReviewFile{Path: name, BeforeDigest: digest}
	}
	for name, digest := range after {
		f := files[name]
		f.Path = name
		f.AfterDigest = digest
		files[name] = f
	}
	result := make([]ReviewFile, 0, len(files))
	for _, f := range files {
		f.Status = "unchanged"
		switch {
		case f.BeforeDigest == "":
			f.Status = "added"
		case f.AfterDigest == "":
			f.Status = "removed"
		case f.BeforeDigest != f.AfterDigest:
			f.Status = "modified"
		}
		f.Module = pathOwner(comparison.After, f.Path)
		if f.Module == "" {
			f.Module = pathOwner(comparison.Before, f.Path)
		}
		result = append(result, f)
	}
	slices.SortFunc(result, func(a, b ReviewFile) int { return strings.Compare(a.Path, b.Path) })
	return result
}

func (s *Service) reviewContent(root, file, digest string) ReviewContent {
	if digest == "" {
		return ReviewContent{Unavailable: "File absent at this revision"}
	}
	source, err := s.ports.Source(root, file, []ir.Evidence{{File: file}})
	if err != nil {
		return ReviewContent{Unavailable: err.Error()}
	}
	if strings.ContainsRune(source, 0) || !utf8.ValidString(source) {
		return ReviewContent{Unavailable: "Binary file; text diff unavailable"}
	}
	return ReviewContent{Text: source}
}

func reviewHotspots(activity ir.Activity) []Hotspot {
	files := map[string]Hotspot{}
	for _, commit := range activity.Commits {
		for name, stats := range commit.Files {
			f := files[name]
			f.Path = name
			f.Touches++
			f.Added += stats.Added
			f.Deleted += stats.Deleted
			f.Binary = f.Binary || stats.Binary
			files[name] = f
		}
	}
	result := make([]Hotspot, 0, len(files))
	for name, f := range files {
		f.Churn = f.Added + f.Deleted
		f.Net = activity.Net[name]
		result = append(result, f)
	}
	slices.SortFunc(result, func(a, b Hotspot) int {
		if a.Touches != b.Touches {
			return b.Touches - a.Touches
		}
		return strings.Compare(a.Path, b.Path)
	})
	return result
}

func (r *ReviewWorkspace) selectObject(id string) {
	owners := ir.Owners(r.Comparison.After)
	graph := r.Comparison.After
	if !slices.ContainsFunc(graph.Symbols, func(s ir.Symbol) bool { return s.ID == id }) {
		graph = r.Comparison.Before
		owners = ir.Owners(graph)
	}
	for _, symbol := range graph.Symbols {
		if symbol.ID == id {
			r.Object = &symbol
			break
		}
	}
	if r.Object == nil {
		return
	}
	for _, rel := range graph.Relations {
		if rel.ToID == id || rel.FromID == id {
			r.References = append(r.References, rel)
		}
	}
	reached := map[string]bool{owners[id]: true}
	for changed := true; changed; {
		changed = false
		for _, rel := range graph.Relations {
			if !ir.IsConsumerRelation(rel) {
				continue
			}
			from, to := owners[rel.FromID], owners[rel.ToID]
			if from != "" && reached[to] && !reached[from] {
				reached[from] = true
				changed = true
			}
		}
	}
	for module := range reached {
		if module != owners[id] {
			r.PossibleImpact = append(r.PossibleImpact, module)
		}
	}
	slices.Sort(r.PossibleImpact)
}

func reviewDigest(f ReviewFile) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(f.BeforeDigest+"\x00"+f.AfterDigest)))
}
func reviewMarks(r ReviewWorkspace, entries []ReviewEntry) []ReviewMark {
	latest := map[string]ReviewEntry{}
	for _, e := range entries {
		if e.BaseRef == r.BaseRef && e.HeadRef == r.HeadRef {
			latest[e.File+"\x00"+e.CommentID] = e
		}
	}
	result := []ReviewMark{}
	for _, e := range latest {
		status := "anchor unavailable"
		for _, f := range r.Files {
			if f.Path == e.File {
				status = "current"
				if e.Digest != reviewDigest(f) {
					status = "needs reconsideration"
				}
				break
			}
		}
		result = append(result, ReviewMark{Entry: e, Status: status})
	}
	slices.SortFunc(result, func(a, b ReviewMark) int {
		return strings.Compare(a.Entry.File+"\x00"+a.Entry.RecordedAt+"\x00"+a.Entry.CommentID, b.Entry.File+"\x00"+b.Entry.RecordedAt+"\x00"+b.Entry.CommentID)
	})
	return result
}

func (s *Service) RecordReview(ctx context.Context, entry ReviewEntry) error {
	if entry.Comment {
		return s.recordComment(ctx, entry)
	}
	if !ir.ValidPath(entry.File) {
		return fmt.Errorf("invalid file review anchor")
	}
	r, err := s.Review(ctx, entry.BaseRef, entry.HeadRef, entry.File, "")
	if err != nil {
		return err
	}
	if entry.Base != r.Comparison.Base || entry.Head != r.Comparison.Head {
		return fmt.Errorf("comparison changed; refresh before saving")
	}
	if len(entry.Note) > 8000 {
		return fmt.Errorf("review note exceeds 8000 bytes")
	}
	for _, f := range r.Files {
		if f.Path == entry.File {
			entry.Digest = reviewDigest(f)
		}
	}
	entry.Context = r.After.Text
	if entry.Context == "" {
		entry.Context = r.Before.Text
	}
	entry.Context = string([]rune(entry.Context)[:min(1200, len([]rune(entry.Context)))])
	entry.RecordedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return s.ports.AppendReview(s.root, entry)
}

// ReviewJSON retains the complete comparison and source evidence, plus only the
// explicitly selected file's bounded full source. Saved notes also retain their
// original bounded source excerpts.
func ReviewJSON(r ReviewWorkspace) string { b, _ := json.MarshalIndent(r, "", "  "); return string(b) }
