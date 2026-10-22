// Package config owns strict YAML validation and explicit module ownership.
package config

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
	"seamgraph/internal/ir"
)

type Config struct {
	Version  int               `yaml:"version"`
	Modules  map[string]Module `yaml:"modules"`
	Policies Policies          `yaml:"policies"`
}
type Module struct {
	Language    string          `yaml:"language"`
	Paths       []string        `yaml:"paths"`
	Seams       map[string]Seam `yaml:"seams,omitempty"`
	ImportRoots []string        `yaml:"importRoots,omitempty"`
	TSConfig    string          `yaml:"tsconfig,omitempty"`
}
type Seam struct {
	Symbols     []string `yaml:"symbols,omitempty"`
	Entrypoints []string `yaml:"entrypoints,omitempty"`
}
type Policies struct {
	Default    string `yaml:"default"`
	Unresolved string `yaml:"unresolved,omitempty"`
	Allow      []Rule `yaml:"allow,omitempty"`
	Deny       []Rule `yaml:"deny,omitempty"`
}
type Rule struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

const Filename = "seamgraph.yaml"

// Document retains the YAML node tree, including comments, for future safe edits.
// P1 never rewrites existing configuration.
type Document struct {
	Config Config
	Node   yaml.Node
}

func Parse(data []byte) (*Document, error) {
	d := &Document{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&d.Node); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, fmt.Errorf("parse YAML: %w", err)
		}
		return nil, fmt.Errorf("only one YAML document is allowed")
	}
	if err := ValidateNodes(&d.Node, map[*yaml.Node]bool{}); err != nil {
		return nil, err
	}
	typed := yaml.NewDecoder(bytes.NewReader(data))
	typed.KnownFields(true)
	if err := typed.Decode(&d.Config); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if d.Config.Policies.Unresolved == "" {
		d.Config.Policies.Unresolved = "error"
	}
	if err := d.Config.Validate(); err != nil {
		return nil, err
	}
	return d, nil
}
func ValidateNodes(n *yaml.Node, seen map[*yaml.Node]bool) error {
	if seen[n] {
		return fmt.Errorf("recursive YAML alias at line %d", n.Line)
	}
	seen[n] = true
	defer delete(seen, n)
	if n.Kind == yaml.AliasNode {
		return fmt.Errorf("YAML aliases are unsupported at line %d; declare values explicitly", n.Line)
	}
	if n.Tag == "!!null" {
		return fmt.Errorf("null configuration value at line %d", n.Line)
	}
	if n.Kind == yaml.MappingNode {
		keys := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode || k.Tag != "!!str" {
				return fmt.Errorf("mapping keys must be strings at line %d", k.Line)
			}
			if keys[k.Value] {
				return fmt.Errorf("duplicate key %q at line %d", k.Value, k.Line)
			}
			keys[k.Value] = true
		}
	}
	for _, child := range n.Content {
		if err := ValidateNodes(child, seen); err != nil {
			return err
		}
	}
	return nil
}
func inside(file, root string) bool {
	return root == "." || file == root || strings.HasPrefix(file, root+"/")
}
func cleanPath(p string) error {
	if strings.ContainsAny(p, "*?[]{}!") {
		return fmt.Errorf("glob paths are unsupported in v1: %q; use literal directory roots", p)
	}
	if !ir.ValidPath(p) {
		return fmt.Errorf("path %q must be a clean project-relative slash path", p)
	}
	return nil
}
func (c Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version %d (expected 1)", c.Version)
	}
	if len(c.Modules) == 0 {
		return fmt.Errorf("at least one module is required")
	}
	if c.Policies.Default != "allow" && c.Policies.Default != "deny" {
		return fmt.Errorf("policies.default must be allow or deny")
	}
	if c.Policies.Unresolved != "error" && c.Policies.Unresolved != "warn" {
		return fmt.Errorf("policies.unresolved must be error or warn")
	}
	roots := map[string]string{}
	names := make([]string, 0, len(c.Modules))
	for name := range c.Modules {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		m := c.Modules[name]
		if !ir.ValidName(name) {
			return fmt.Errorf("invalid module name %q", name)
		}
		if !slices.Contains([]string{"go", "typescript", "python"}, m.Language) {
			return fmt.Errorf("module %s: unsupported language %q", name, m.Language)
		}
		if len(m.Paths) == 0 {
			return fmt.Errorf("module %s needs paths", name)
		}
		for _, p := range m.Paths {
			if err := cleanPath(p); err != nil && p != "." {
				return fmt.Errorf("module %s: %w", name, err)
			}
			rootNames := make([]string, 0, len(roots))
			for r := range roots {
				rootNames = append(rootNames, r)
			}
			slices.Sort(rootNames)
			for _, r := range rootNames {
				if inside(p, r) || inside(r, p) {
					return fmt.Errorf("overlapping module roots %q (%s) and %q (%s)", r, roots[r], p, name)
				}
			}
			roots[p] = name
		}
		if m.TSConfig != "" {
			if m.Language != "typescript" {
				return fmt.Errorf("module %s: tsconfig applies only to typescript", name)
			}
			if err := cleanPath(m.TSConfig); err != nil {
				return err
			}
		}
		if len(m.ImportRoots) > 0 && m.Language != "python" {
			return fmt.Errorf("module %s: importRoots is supported only for python", name)
		}
		for _, p := range m.ImportRoots {
			if err := cleanPath(p); err != nil {
				return err
			}
		}
		seamNames := make([]string, 0, len(m.Seams))
		for n := range m.Seams {
			seamNames = append(seamNames, n)
		}
		slices.Sort(seamNames)
		for _, n := range seamNames {
			seam := m.Seams[n]
			if !ir.ValidName(n) {
				return fmt.Errorf("invalid seam name %q in module %s", n, name)
			}
			if len(seam.Symbols) == 0 && len(seam.Entrypoints) == 0 {
				return fmt.Errorf("seam %s.%s needs symbols or entrypoints", name, n)
			}
			if m.Language == "go" && len(seam.Entrypoints) > 0 {
				return fmt.Errorf("Go seam %s.%s must declare symbols, not entrypoints", name, n)
			}
			seen := map[string]bool{}
			for _, sym := range seam.Symbols {
				if strings.TrimSpace(sym) == "" || strings.TrimSpace(sym) != sym || seen[sym] {
					return fmt.Errorf("invalid or duplicate symbol %q in %s.%s", sym, name, n)
				}
				seen[sym] = true
			}
			seen = map[string]bool{}
			for _, p := range seam.Entrypoints {
				if err := cleanPath(p); err != nil {
					return err
				}
				owned := false
				for _, r := range m.Paths {
					if inside(p, r) {
						owned = true
					}
				}
				if !owned || seen[p] {
					return fmt.Errorf("entrypoint %q must belong uniquely to module %s", p, name)
				}
				seen[p] = true
				if m.Language == "python" && !strings.HasSuffix(p, ".py") {
					return fmt.Errorf("Python entrypoint %q must be a .py file", p)
				}
			}
		}
	}
	seenRules := map[string]bool{}
	for _, group := range []struct {
		name  string
		rules []Rule
	}{{"allow", c.Policies.Allow}, {"deny", c.Policies.Deny}} {
		for _, r := range group.rules {
			if _, ok := c.Modules[r.From]; !ok {
				return fmt.Errorf("%s rule: unknown consumer %q", group.name, r.From)
			}
			target := strings.Split(r.To, ".")
			if len(target) > 2 || len(target) == 0 {
				return fmt.Errorf("invalid target %q", r.To)
			}
			m, ok := c.Modules[target[0]]
			if !ok {
				return fmt.Errorf("%s rule: unknown provider %q", group.name, r.To)
			}
			if len(target) == 2 {
				if _, ok := m.Seams[target[1]]; !ok {
					return fmt.Errorf("%s rule: unknown seam %q", group.name, r.To)
				}
			}
			k := r.From + "->" + r.To
			if seenRules[k] {
				return fmt.Errorf("duplicate or conflicting rule %s", k)
			}
			seenRules[k] = true
		}
	}
	return nil
}

// Owner resolves only normalized project-relative paths. Empty ID with nil error
// explicitly means unowned. Symlinks must be resolved by adapters before calling.
func (c Config) Owner(file string) (string, error) {
	if err := cleanPath(file); err != nil {
		return "", err
	}
	owner := ""
	for name, m := range c.Modules {
		for _, root := range m.Paths {
			if inside(file, root) {
				if owner != "" && owner != ir.ModuleID(name) {
					return "", fmt.Errorf("ambiguous ownership for %s", file)
				}
				owner = ir.ModuleID(name)
			}
		}
	}
	return owner, nil
}

// IRModules returns declared ownership and seams only, never observed relations
// or symbols. Adapters must resolve all configured contract member selectors.
func (c Config) IRModules() ([]ir.Module, []ir.Seam) {
	modules := []ir.Module{}
	seams := []ir.Seam{}
	names := make([]string, 0, len(c.Modules))
	for n := range c.Modules {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		m := c.Modules[n]
		node := ir.Module{ID: ir.ModuleID(n), DisplayName: n, SourceRoots: append([]string{}, m.Paths...), Seams: []string{}}
		slices.Sort(node.SourceRoots)
		for seam := range m.Seams {
			id := ir.SeamID(n, seam)
			node.Seams = append(node.Seams, id)
			seams = append(seams, ir.Seam{ID: id, ModuleID: node.ID, ContractMembers: []string{}})
		}
		slices.Sort(node.Seams)
		modules = append(modules, node)
	}
	slices.SortFunc(seams, func(a, b ir.Seam) int { return strings.Compare(a.ID, b.ID) })
	return modules, seams
}
func (c Config) YAML() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return yaml.Marshal(c)
}
