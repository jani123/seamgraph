package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"seamgraph/internal/ir"
)

type CodeObject struct {
	Matching     string    `json:"matching"`
	PreviousFile string    `json:"previousFile,omitempty"`
	Symbol       ir.Symbol `json:"symbol"`
	Status       string    `json:"status"`
	Seams        []string  `json:"seams"`
}
type CodeBrowser struct {
	AfterRelations       []ir.Relation     `json:"afterRelations,omitempty"`
	BeforeRelations      []ir.Relation     `json:"beforeRelations,omitempty"`
	Options              map[string]string `json:"options,omitempty"`
	Range                *ir.ReviewRange   `json:"range,omitempty"`
	Marks                []ReviewMark      `json:"marks,omitempty"`
	ReviewBaseRef        string            `json:"reviewBaseRef,omitempty"`
	ReviewHeadRef        string            `json:"reviewHeadRef,omitempty"`
	ContentRevision      string            `json:"contentRevision"`
	RelationshipRevision string            `json:"relationshipRevision"`
	Revision             string            `json:"revision"`
	Base                 string            `json:"base,omitempty"`
	Files                []ReviewFile      `json:"files"`
	Objects              []CodeObject      `json:"objects"`
	Relations            []ir.Relation     `json:"relations"`
	Diagnostics          []ir.Diagnostic   `json:"diagnostics"`
	Issues               []string          `json:"issues"`
	Selected             string            `json:"selected,omitempty"`
	Object               string            `json:"object,omitempty"`
	Content              ReviewContent     `json:"content"`
	Before               ReviewContent     `json:"before"`
	After                ReviewContent     `json:"after"`
}
type codeCache struct {
	mu      sync.Mutex
	entries map[string]ir.CodeIndex
}

func (s *Service) codeIndex(ctx context.Context, root, identity string, files map[string]string) (ir.CodeIndex, error) {
	data, _ := json.Marshal(files)
	key := identity + ":" + fmt.Sprintf("%x", sha256.Sum256(data))
	s.code.mu.Lock()
	defer s.code.mu.Unlock()
	if index, ok := s.code.entries[key]; ok {
		return index, nil
	}
	index, err := s.ports.Index(ctx, root, files)
	if err == nil {
		if len(s.code.entries) >= 8 || s.code.entries == nil {
			s.code.entries = map[string]ir.CodeIndex{}
		}
		s.code.entries[key] = index
	}
	return index, err
}
func (s *Service) readCode(ctx context.Context, revision, base, file, object string) (CodeBrowser, error) {
	root := s.root
	cleanup := func() {}
	identity := "Working directory"
	if revision != "" {
		var err error
		root, identity, cleanup, err = s.ports.Extract(ctx, s.root, revision)
		if err != nil {
			return CodeBrowser{}, err
		}
	}
	defer cleanup()
	inventory := s.ports.BrowseInventory
	if revision != "" {
		inventory = s.ports.Inventory
	}
	files, err := inventory(root)
	if err != nil {
		return CodeBrowser{}, err
	}
	index, err := s.codeIndex(ctx, root, identity, files)
	if err != nil {
		return CodeBrowser{}, err
	}
	result := codeBrowser(index, files, identity)
	result.ContentRevision = identity
	result.RelationshipRevision = identity
	if base != "" {
		before, err := s.readCode(ctx, base, "", "", "")
		if err != nil {
			return result, err
		}
		result.Base = before.Revision
		result = compareCode(before, result)
	}
	result.Selected = file
	result.Object = object
	if object != "" {
		for _, o := range result.Objects {
			if o.Symbol.ID == object && o.Symbol.Definition != nil {
				result.Selected = o.Symbol.Definition.Location.File
				break
			}
		}
	}
	result, err = s.codeContent(ctx, root, files, base, result)
	if err != nil {
		return result, err
	}
	return result, nil
}
func (s *Service) codeContent(ctx context.Context, root string, files map[string]string, base string, result CodeBrowser) (CodeBrowser, error) {
	removed := false
	for _, object := range result.Objects {
		if object.Symbol.ID == result.Object && object.Status == "removed" {
			removed = true
		}
	}
	if result.Selected == "" {
		return result, nil
	}
	if base != "" && (removed || files[result.Selected] == "") {
		previous, err := s.readCode(ctx, base, "", basePath(result), "")
		if err != nil {
			return result, err
		}
		if previous.Revision != result.Base {
			return result, fmt.Errorf("comparison base moved during source selection; refresh")
		}
		result.Before = previous.Content
		result.Content = previous.Content
		result.ContentRevision = previous.Revision
		if removed {
			result.Relations = previous.Relations
			result.RelationshipRevision = previous.Revision
		}
	} else if files[result.Selected] != "" {
		result.Content = s.reviewContent(root, result.Selected, files[result.Selected])
	} else {
		return result, fmt.Errorf("unknown code file")
	}
	result.After = result.Content
	if removed && files[result.Selected] != "" {
		result.After = s.reviewContent(root, result.Selected, files[result.Selected])
	}
	for _, file := range result.Files {
		if file.Path == result.Selected && file.AfterDigest == "" {
			result.After = ReviewContent{Unavailable: "File absent at this revision"}
		}
	}
	if base != "" && result.Before.Text == "" && result.Before.Unavailable == "" {
		return s.codeBaseline(ctx, base, result)
	}
	return result, nil
}

func (s *Service) codeBaseline(ctx context.Context, base string, result CodeBrowser) (CodeBrowser, error) {
	result.Before = ReviewContent{Unavailable: "File absent at this revision"}
	for _, file := range result.Files {
		if file.Path != result.Selected || file.BeforeDigest == "" {
			continue
		}
		before, err := s.readCode(ctx, base, "", basePath(result), "")
		if err != nil {
			return result, err
		}
		if before.Revision != result.Base {
			return result, fmt.Errorf("comparison base moved during source selection; refresh")
		}
		result.Before = before.Content
	}
	return result, nil
}

func codeBrowser(index ir.CodeIndex, files map[string]string, revision string) CodeBrowser {
	result := CodeBrowser{Revision: revision, Files: []ReviewFile{}, Objects: []CodeObject{}, Issues: index.Issues, Diagnostics: index.Facts.Diagnostics, Relations: []ir.Relation{}}
	ids := map[string]string{}
	for _, symbol := range index.Facts.Symbols {
		owner := "module:unassigned"
		if symbol.Definition != nil {
			owner = codeOwner(index.DeclaredModules, symbol.Definition.Location.File)
		}
		_, raw, _ := strings.Cut(symbol.ID, "symbol:unassigned:")
		ids[symbol.ID] = "symbol:" + strings.TrimPrefix(owner, "module:") + ":" + raw
	}
	for _, symbol := range index.Facts.Symbols {
		original := symbol.ID
		symbol.ID = ids[original]
		symbol.ModuleID = "module:unassigned"
		if symbol.Definition != nil {
			definition := *symbol.Definition
			definition.ParentID = ids[definition.ParentID]
			symbol.Definition = &definition
			symbol.ModuleID = codeOwner(index.DeclaredModules, definition.Location.File)
		}
		object := CodeObject{Symbol: symbol, Status: "unchanged", Matching: "stable ID", Seams: []string{}}
		for _, seam := range index.DeclaredSeams {
			if slices.Contains(seam.ContractMembers, symbol.ID) {
				object.Seams = append(object.Seams, seam.ID)
			}
		}
		result.Objects = append(result.Objects, object)
	}
	for file, digest := range files {
		result.Files = append(result.Files, ReviewFile{Path: file, Configuration: file == ConfigFilename, Status: "unchanged", Module: codeOwner(index.DeclaredModules, file), AfterDigest: digest})
	}
	slices.SortFunc(result.Files, func(a, b ReviewFile) int { return strings.Compare(a.Path, b.Path) })
	result.Relations = codeRelations(index, ids)
	return result
}
func codeOwner(modules []ir.Module, file string) string {
	for _, module := range modules {
		for _, root := range module.SourceRoots {
			if root == "." || file == root || strings.HasPrefix(file, root+"/") {
				return module.ID
			}
		}
	}
	return "module:unassigned"
}
func codeRelations(index ir.CodeIndex, ids map[string]string) []ir.Relation {
	result := []ir.Relation{}
	for _, relation := range index.Facts.Relations {
		target := ids[relation.ToID]
		if target == "" {
			continue
		}
		for _, e := range relation.Evidence {
			source := ids[relation.FromID]
			if source == "" {
				source = ids[e.SourceID]
			}
			if source == "" {
				source = codeOwner(index.DeclaredModules, e.File)
			}
			e.SourceID = ids[e.SourceID]
			result = append(result, ir.Relation{ID: ir.RelationID(relation.Kind, source, target), FromID: source, ToID: target, Kind: relation.Kind, Evidence: []ir.Evidence{e}})
		}
	}
	return result
}
func compareCode(before, after CodeBrowser) CodeBrowser {
	after.BeforeRelations = before.Relations
	after.AfterRelations = after.Relations
	symbols := map[string]CodeObject{}
	for _, o := range before.Objects {
		symbols[o.Symbol.ID] = o
	}
	for i, o := range after.Objects {
		old, ok := symbols[o.Symbol.ID]
		if !ok {
			after.Objects[i].Status = "added"
			after.Objects[i].Matching = "unmatched ID; a renamed/moved declaration may be the same object"
		} else if codeSignature(old.Symbol) != codeSignature(o.Symbol) {
			after.Objects[i].Status = "modified"
		}
		if ok && old.Symbol.Definition != nil && o.Symbol.Definition != nil && old.Symbol.Definition.Location.File != o.Symbol.Definition.Location.File {
			after.Objects[i].Status = "modified"
			after.Objects[i].PreviousFile = old.Symbol.Definition.Location.File
		}
		delete(symbols, o.Symbol.ID)
	}
	for _, old := range symbols {
		old.Status = "removed"
		old.Matching = "unmatched ID; a renamed/moved declaration may be the same object"
		after.Objects = append(after.Objects, old)
	}
	beforeFiles := map[string]string{}
	afterFiles := map[string]string{}
	for _, f := range before.Files {
		beforeFiles[f.Path] = f.AfterDigest
	}
	for _, f := range after.Files {
		afterFiles[f.Path] = f.AfterDigest
	}
	owners := map[string]string{}
	for _, file := range before.Files {
		owners[file.Path] = file.Module
	}
	for _, file := range after.Files {
		owners[file.Path] = file.Module
	}
	after.Files = reviewFiles(beforeFiles, afterFiles, Result{})
	for i := range after.Files {
		after.Files[i].Module = owners[after.Files[i].Path]
	}
	slices.SortFunc(after.Objects, func(a, b CodeObject) int { return strings.Compare(a.Symbol.ID, b.Symbol.ID) })
	return after
}
func codeSignature(symbol ir.Symbol) string {
	if symbol.Definition != nil {
		definition := *symbol.Definition
		definition.Location = ir.Evidence{}
		symbol.Definition = &definition
	}
	data, _ := json.Marshal(symbol)
	return string(data)
}

func (s *Service) Code(ctx context.Context, revision, base, file, object string) (CodeBrowser, error) {
	selection, err := s.reviewRange(ctx, base, revision)
	if err != nil {
		return CodeBrowser{}, err
	}
	actualBase, actualHead := base, revision
	if selection != nil {
		actualBase, actualHead = selection.Base, selection.Head
	}
	code, err := s.readCode(ctx, actualHead, actualBase, file, object)
	if err != nil {
		return code, err
	}
	code.Range = selection
	code.ReviewBaseRef, code.ReviewHeadRef = base, revision
	return s.codeComments(code, base, revision)
}

func basePath(code CodeBrowser) string {
	for _, file := range code.Files {
		if file.Path == code.Selected && file.OldPath != "" {
			return file.OldPath
		}
	}
	return code.Selected
}
