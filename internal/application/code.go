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
	Symbol ir.Symbol `json:"symbol"`
	Status string    `json:"status"`
	Seams  []string  `json:"seams"`
}
type CodeBrowser struct {
	ReviewBaseRef        string          `json:"reviewBaseRef,omitempty"`
	ReviewHeadRef        string          `json:"reviewHeadRef,omitempty"`
	ContentRevision      string          `json:"contentRevision"`
	RelationshipRevision string          `json:"relationshipRevision"`
	Revision             string          `json:"revision"`
	Base                 string          `json:"base,omitempty"`
	Files                []ReviewFile    `json:"files"`
	Objects              []CodeObject    `json:"objects"`
	Relations            []ir.Relation   `json:"relations"`
	Diagnostics          []ir.Diagnostic `json:"diagnostics"`
	Issues               []string        `json:"issues"`
	Selected             string          `json:"selected,omitempty"`
	Object               string          `json:"object,omitempty"`
	Content              ReviewContent   `json:"content"`
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
func (s *Service) Code(ctx context.Context, revision, base, file, object string) (CodeBrowser, error) {
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
	files, err := s.ports.BrowseInventory(root)
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
		before, err := s.Code(ctx, base, "", "", "")
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
	return s.codeContent(ctx, root, files, base, result)
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
		previous, err := s.Code(ctx, base, "", result.Selected, "")
		if err != nil {
			return result, err
		}
		if previous.Revision != result.Base {
			return result, fmt.Errorf("comparison base moved during source selection; refresh")
		}
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
		object := CodeObject{Symbol: symbol, Status: "unchanged", Seams: []string{}}
		for _, seam := range index.DeclaredSeams {
			if slices.Contains(seam.ContractMembers, symbol.ID) {
				object.Seams = append(object.Seams, seam.ID)
			}
		}
		result.Objects = append(result.Objects, object)
	}
	for file, digest := range files {
		result.Files = append(result.Files, ReviewFile{Path: file, Status: "unchanged", Module: codeOwner(index.DeclaredModules, file), AfterDigest: digest})
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
	symbols := map[string]CodeObject{}
	for _, o := range before.Objects {
		symbols[o.Symbol.ID] = o
	}
	for i, o := range after.Objects {
		old, ok := symbols[o.Symbol.ID]
		if !ok {
			after.Objects[i].Status = "added"
		} else if codeSignature(old.Symbol) != codeSignature(o.Symbol) {
			after.Objects[i].Status = "modified"
		}
		delete(symbols, o.Symbol.ID)
	}
	for _, old := range symbols {
		old.Status = "removed"
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
