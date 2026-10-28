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
	Configuration bool   `json:"configuration,omitempty"`
	OldPath       string `json:"oldPath,omitempty"`
	Path          string `json:"path"`
	Status        string `json:"status"`
	Module        string `json:"module,omitempty"`
	BeforeDigest  string `json:"beforeDigest,omitempty"`
	AfterDigest   string `json:"afterDigest,omitempty"`
}
type ReviewEntry struct {
	Deleted     bool   `json:"deleted,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	EditVersion string `json:"-"`
	Checklist   bool   `json:"checklist,omitempty"`
	Comment     bool   `json:"comment,omitempty"`
	CommentID   string `json:"commentId,omitempty"`
	Side        string `json:"side,omitempty"`
	Line        int    `json:"line,omitempty"`
	BaseRef     string `json:"baseRef"`
	HeadRef     string `json:"headRef"`
	Base        string `json:"base"`
	Head        string `json:"head"`
	File        string `json:"file"`
	Digest      string `json:"digest"`
	Context     string `json:"context"`
	Note        string `json:"note"`
	Viewed      bool   `json:"viewed"`
	Resolved    bool   `json:"resolved"`
	RecordedAt  string `json:"recordedAt"`
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
	Paths   []string     `json:"paths,omitempty"`
	Path    string       `json:"path"`
	Touches int          `json:"touches"`
	Added   int          `json:"added"`
	Deleted int          `json:"deleted"`
	Churn   int          `json:"churn"`
	Binary  bool         `json:"binary"`
	Net     ir.LineStats `json:"net"`
}
type ReviewWorkspace struct {
	Summary           ReviewSummary     `json:"summary"`
	Options           map[string]string `json:"options,omitempty"`
	Range             *ir.ReviewRange   `json:"range,omitempty"`
	ArchitectureIssue string            `json:"architectureIssue,omitempty"`
	Code              CodeBrowser       `json:"code"`
	PathFilter        string            `json:"pathFilter"`
	TypeFilter        string            `json:"typeFilter"`
	BaseRef           string            `json:"baseRef"`
	HeadRef           string            `json:"headRef"`
	Comparison        Result            `json:"comparison"`
	Files             []ReviewFile      `json:"files"`
	Activity          ir.Activity       `json:"activity"`
	Hotspots          []Hotspot         `json:"hotspots"`
	Selected          string            `json:"selected,omitempty"`
	Before            ReviewContent     `json:"before"`
	After             ReviewContent     `json:"after"`
	Object            *ir.Symbol        `json:"object,omitempty"`
	References        []ir.Relation     `json:"references"`
	PossibleImpact    []string          `json:"possibleImpact"`
	Marks             []ReviewMark      `json:"marks"`
}

func (s *Service) Review(ctx context.Context, base, head, file, object string) (ReviewWorkspace, error) {
	r := ReviewWorkspace{BaseRef: base, HeadRef: head, Selected: file}
	code, err := s.Code(ctx, head, base, file, object)
	if err != nil {
		return r, err
	}
	r.Code = code
	r.Range = code.Range
	comparison, compareErr := s.Compare(ctx, treeRef(code.Base), treeRef(code.Revision))
	if compareErr != nil {
		r.ArchitectureIssue = "Architecture context unavailable: " + compareErr.Error()
		comparison = Result{SchemaVersion: 1, Base: code.Base, Head: code.Revision}
	}
	if comparison.Base != code.Base || comparison.Head != code.Revision {
		return r, fmt.Errorf("comparison changed during review; refresh")
	}
	r.Comparison = comparison
	if r.ArchitectureIssue != "" {
		r.Code.Diagnostics = append(slices.Clone(r.Code.Diagnostics), ir.Diagnostic{Severity: "warning", Code: "ARCHITECTURE_UNAVAILABLE", Message: r.ArchitectureIssue})
	}
	r.Code.ReviewBaseRef = base
	r.Code.ReviewHeadRef = head
	r.Selected = r.Code.Selected
	if err := s.reviewFiles(ctx, &r); err != nil {
		return r, err
	}
	historyBase := comparison.Base
	if r.Range != nil {
		historyBase = r.Range.TargetRevision
	}
	r.Activity = s.reviewActivity(ctx, historyBase, comparison.Head, r.Selected)
	if historyBase != comparison.Base {
		net, netErr := s.ports.NetStats(ctx, s.root, comparison.Base, comparison.Head)
		if netErr != nil {
			return r, netErr
		}
		r.Activity.Net = net
	}
	r.Hotspots = reviewHotspots(r.Activity)
	r.selectObject(object)
	entries, err := s.ports.ReadReview(s.root)
	if err != nil {
		return r, err
	}
	r.Marks = reviewMarks(r, entries)
	r.Summary = SummarizeReview(r)
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
	r.Before = s.reviewContent(before, basePath(r.Code), a[basePath(r.Code)])
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
		f.Configuration = f.Path == ConfigFilename
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
	annotateMoves(result)
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
			key := e.File + "\x00" + e.CommentID
			if e.Checklist {
				key = "checklist"
			}
			latest[key] = e
		}
	}
	result := []ReviewMark{}
	for _, e := range latest {
		if e.Comment && e.Deleted {
			continue
		}
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
		if e.Checklist {
			status = "current"
			if e.Base != r.Comparison.Base || e.Head != r.Comparison.Head {
				status = "needs reconsideration"
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
	if entry.Checklist {
		return s.recordChecklist(ctx, entry)
	}
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

// Exact moves have evidence in both trees. Edited moves deliberately remain
// independent additions/deletions rather than guessing object identity.
func annotateMoves(files []ReviewFile) {
	before, after := map[string]string{}, map[string]string{}
	for _, f := range files {
		if f.BeforeDigest != "" {
			before[f.Path] = f.BeforeDigest
		}
		if f.AfterDigest != "" {
			after[f.Path] = f.AfterDigest
		}
	}
	for _, change := range fileChanges(before, after) {
		if change.OldPath == "" {
			continue
		}
		for i := range files {
			if files[i].Path == change.Path {
				files[i].OldPath = change.OldPath
				files[i].BeforeDigest = before[change.OldPath]
				files[i].Status = "renamed"
			}
		}
	}
}

func (s *Service) recordChecklist(ctx context.Context, entry ReviewEntry) error {
	if len(entry.Note) > 8000 {
		return fmt.Errorf("checklist exceeds 8000 bytes")
	}
	code, err := s.Code(ctx, entry.HeadRef, entry.BaseRef, "", "")
	if err != nil {
		return err
	}
	if entry.Base != code.Base || entry.Head != code.Revision {
		return fmt.Errorf("comparison changed; refresh before saving")
	}
	entry.File = ""
	entry.RecordedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return s.ports.AppendReview(s.root, entry)
}

func treeRef(identity string) string {
	if strings.HasPrefix(identity, "WORKTREE:") {
		return "WORKTREE"
	}
	return identity
}

func (s *Service) reviewActivity(ctx context.Context, base, head, file string) ir.Activity {
	activity, err := s.ports.Activity(ctx, s.root, base, head, file)
	if err != nil {
		return ir.Activity{Base: base, Head: head, Policy: "base..head; merges excluded; no rename following; churn = additions + deletions", Unavailable: "Commit history unavailable: " + err.Error(), Commits: []ir.ReviewCommit{}, Net: map[string]ir.LineStats{}}
	}
	return activity
}
