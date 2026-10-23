package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"seamgraph/internal/config"
	"seamgraph/internal/ir"
)

// Builder provides common ownership and evidence plumbing inside the adapter boundary.
type Builder struct {
	Root    string
	Config  config.Config
	Partial ir.PartialSnapshot
	Symbols map[string]ir.Symbol
	Members map[string]map[string]bool
}

func NewBuilder(root string, c config.Config, language string) (*Builder, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	b := &Builder{Root: root, Config: c, Symbols: map[string]ir.Symbol{}, Members: map[string]map[string]bool{}}
	modules, seams := c.IRModules()
	selected := map[string]bool{}
	for name, m := range c.Modules {
		if m.Language == language {
			selected[ir.ModuleID(name)] = true
			for _, r := range m.Paths {
				if err := validateModuleRoot(root, name, r); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, m := range modules {
		if selected[m.ID] {
			b.Partial.Modules = append(b.Partial.Modules, m)
		}
	}
	for _, s := range seams {
		if selected[s.ModuleID] {
			b.Partial.Seams = append(b.Partial.Seams, s)
			b.Members[s.ID] = map[string]bool{}
		}
	}
	return b, nil
}
func (b *Builder) Relative(file string) (string, error) {
	resolved, err := filepath.EvalSymlinks(file)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(b.Root, resolved)
	rel = filepath.ToSlash(rel)
	if err != nil || !ir.ValidPath(rel) {
		return "", fmt.Errorf("source escapes project: %s", file)
	}
	return rel, nil
}
func (b *Builder) Owner(file string) string {
	rel, err := b.Relative(file)
	if err != nil {
		return ""
	}
	owner, _ := b.Config.Owner(rel)
	return owner
}
func (b *Builder) Diagnostic(code, message string, evidence ...ir.Evidence) {
	b.Partial.Diagnostics = append(b.Partial.Diagnostics, ir.Diagnostic{Code: code, Severity: "warning", Message: message, EntityIDs: []string{}, Evidence: evidence})
}
func (b *Builder) AddSymbol(s ir.Symbol) { b.Symbols[s.ID] = s }
func (b *Builder) Relation(kind, from, to string, e ir.Evidence, bypass bool) {
	b.Partial.Relations = append(b.Partial.Relations, ir.Relation{ID: ir.RelationID(kind, from, to), FromID: from, ToID: to, Kind: kind, Evidence: []ir.Evidence{e}, BypassesSeam: bypass})
}
func (b *Builder) Finish() ir.PartialSnapshot {
	for _, s := range b.Symbols {
		b.Partial.Symbols = append(b.Partial.Symbols, s)
	}
	for i := range b.Partial.Seams {
		for id := range b.Members[b.Partial.Seams[i].ID] {
			b.Partial.Seams[i].ContractMembers = append(b.Partial.Seams[i].ContractMembers, id)
		}
	}
	return b.Partial
}

func validateModuleRoot(root, name, r string) error {
	p := filepath.Join(root, filepath.FromSlash(r))
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return fmt.Errorf("module %s root: %w", name, err)
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("module root escapes project: %s", r)
	}
	if filepath.Clean(p) != filepath.Clean(resolved) {
		return fmt.Errorf("symlinked module roots are unsupported: %s", r)
	}
	err = filepath.WalkDir(p, validateSourceEntry)
	if err != nil {
		return err
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("module root must be a directory: %s", r)
	}
	return nil
}

func validateSourceEntry(file string, d os.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if d.IsDir() && (d.Name() == "node_modules" || d.Name() == ".venv" || d.Name() == ".git" || d.Name() == "__pycache__" || d.Name() == ".build" || d.Name() == ".seamgraph" || d.Name() == ".devbox" || d.Name() == ".codex-local") {
		return filepath.SkipDir
	}
	if d.Type()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlinked source is unsupported: %s", file)
	}
	return nil
}
