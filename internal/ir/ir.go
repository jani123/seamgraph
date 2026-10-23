// Package ir defines SeamGraph's versioned language-neutral architecture boundary.
package ir

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
)

const SchemaVersion = 1

type Module struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	SourceRoots []string `json:"sourceRoots"`
	Seams       []string `json:"seams"`
}
type Seam struct {
	ID              string   `json:"id"`
	ModuleID        string   `json:"moduleId"`
	ContractMembers []string `json:"contractMembers"`
}
type Symbol struct {
	ID               string `json:"id"`
	ModuleID         string `json:"moduleId"`
	Kind             string `json:"kind"`
	DisplaySignature string `json:"displaySignature"`
	Visibility       string `json:"visibility"`
}
type Evidence struct {
	File        string `json:"file" yaml:"file"`
	StartLine   int    `json:"startLine" yaml:"startLine"`
	StartColumn int    `json:"startColumn" yaml:"startColumn"`
	EndLine     int    `json:"endLine" yaml:"endLine"`
	EndColumn   int    `json:"endColumn" yaml:"endColumn"`
	Confidence  string `json:"confidence" yaml:"confidence"` // exact or best-effort
	Resolution  string `json:"resolution" yaml:"resolution"` // resolved, unresolved, dynamic
}
type Relation struct {
	ID             string     `json:"id"`
	FromID         string     `json:"fromId"`
	ToID           string     `json:"toId"`
	Kind           string     `json:"kind"`
	BypassesSeam   bool       `json:"bypassesSeam,omitempty"`
	BypassEvidence []Evidence `json:"bypassEvidence,omitempty"`
	Evidence       []Evidence `json:"evidence"`
}
type Diagnostic struct {
	Code      string     `json:"code"`
	Severity  string     `json:"severity"`
	EntityIDs []string   `json:"entityIds"`
	Evidence  []Evidence `json:"evidence"`
	Message   string     `json:"message"`
}
type Snapshot struct {
	SchemaVersion int          `json:"schemaVersion"`
	Modules       []Module     `json:"modules"`
	Seams         []Seam       `json:"seams"`
	Symbols       []Symbol     `json:"symbols"`
	Relations     []Relation   `json:"relations"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
}

// PartialSnapshot is the adapter interchange type. References may resolve in other
// partials; Merge validates the complete graph, not individual partials.
type PartialSnapshot struct {
	Modules     []Module
	Seams       []Seam
	Symbols     []Symbol
	Relations   []Relation
	Diagnostics []Diagnostic
}

var namePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func ValidName(s string) bool           { return namePattern.MatchString(s) }
func ModuleID(name string) string       { return "module:" + name }
func SeamID(module, seam string) string { return "seam:" + module + "." + seam }
func SymbolID(module, canonicalPath string) string {
	return "symbol:" + module + ":" + url.PathEscape(canonicalPath)
}
func RelationID(kind, from, to string) string {
	return "edge:" + kind + ":" + url.QueryEscape(from) + "->" + url.QueryEscape(to)
}

// ViolationID is independent of source location. Rule IDs must also be stable.
func ViolationID(ruleID, relationID string) string {
	sum := sha256.Sum256([]byte(ruleID + "\x00" + relationID))
	return "violation:" + hex.EncodeToString(sum[:])
}
func ValidPath(p string) bool {
	return p != "" && p != "." && !strings.ContainsAny(p, "\\:\x00") && !strings.HasPrefix(p, "/") && p != ".." && !strings.HasPrefix(p, "../") && path.Clean(p) == p
}
func validateEvidence(e Evidence) error {
	if !ValidPath(e.File) || e.StartLine < 1 || e.StartColumn < 1 || e.EndLine < e.StartLine || e.EndColumn < 1 || (e.EndLine == e.StartLine && e.EndColumn < e.StartColumn) {
		return fmt.Errorf("invalid evidence location: %+v", e)
	}
	if e.Confidence != "exact" && e.Confidence != "best-effort" {
		return fmt.Errorf("invalid confidence %q", e.Confidence)
	}
	if e.Resolution != "resolved" && e.Resolution != "unresolved" && e.Resolution != "dynamic" {
		return fmt.Errorf("invalid resolution %q", e.Resolution)
	}
	return nil
}
func (s Snapshot) Validate() error {
	if s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported IR schema version %d", s.SchemaVersion)
	}
	ids := validationIDs{}
	modules, seams, symbols := map[string]Module{}, map[string]Seam{}, map[string]Symbol{}
	checks := []func() error{
		func() error { return validateModules(s, ids, modules) },
		func() error { return validateRootOwnership(s) },
		func() error { return validateSeams(s, ids, modules, seams) },
		func() error { return validateSymbols(s, ids, modules, symbols) },
		func() error { return validateContractMembers(s, seams, symbols) },
		func() error { return validateRelations(s, ids) },
		func() error { return validateDiagnostics(s, ids) },
	}
	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

type validationIDs map[string]string

func (ids validationIDs) add(id, kind string) error {
	if id == "" {
		return fmt.Errorf("empty %s ID", kind)
	}
	if _, ok := ids[id]; ok {
		return fmt.Errorf("duplicate ID %q", id)
	}
	ids[id] = kind
	return nil
}

func validateModules(s Snapshot, ids validationIDs, modules map[string]Module) error {
	for _, m := range s.Modules {
		if !strings.HasPrefix(m.ID, "module:") || !ValidName(strings.TrimPrefix(m.ID, "module:")) {
			return fmt.Errorf("invalid module ID %q", m.ID)
		}
		if err := ids.add(m.ID, "module"); err != nil {
			return err
		}
		modules[m.ID] = m
		if m.DisplayName == "" || len(m.SourceRoots) == 0 {
			return fmt.Errorf("module %s needs display name and source roots", m.ID)
		}
		for _, p := range m.SourceRoots {
			if !ValidRoot(p) {
				return fmt.Errorf("invalid source root %q", p)
			}
		}
		if hasDuplicate(m.SourceRoots) || hasDuplicate(m.Seams) {
			return fmt.Errorf("duplicate roots or seams in %s", m.ID)
		}
	}
	return nil
}

func validateRootOwnership(s Snapshot) error {
	// Root ownership must also be unambiguous when graphs arrive from adapters.
	type ownedRoot struct{ root, module string }
	roots := []ownedRoot{}
	for _, m := range s.Modules {
		for _, root := range m.SourceRoots {
			for _, prior := range roots {
				if root == "." || prior.root == "." || root == prior.root || strings.HasPrefix(root, prior.root+"/") || strings.HasPrefix(prior.root, root+"/") {
					return fmt.Errorf("overlapping source roots %q (%s) and %q (%s)", prior.root, prior.module, root, m.ID)
				}
			}
			roots = append(roots, ownedRoot{root, m.ID})
		}
	}
	return nil
}

func validateSeams(s Snapshot, ids validationIDs, modules map[string]Module, seams map[string]Seam) error {
	for _, x := range s.Seams {
		m, ok := modules[x.ModuleID]
		if !ok {
			return fmt.Errorf("unknown module %q", x.ModuleID)
		}
		prefix := "seam:" + strings.TrimPrefix(x.ModuleID, "module:") + "."
		if !strings.HasPrefix(x.ID, prefix) || !ValidName(strings.TrimPrefix(x.ID, prefix)) {
			return fmt.Errorf("invalid seam ID %q", x.ID)
		}
		if !slices.Contains(m.Seams, x.ID) {
			return fmt.Errorf("module %s does not list seam %s", m.ID, x.ID)
		}
		if err := ids.add(x.ID, "seam"); err != nil {
			return err
		}
		seams[x.ID] = x
		if hasDuplicate(x.ContractMembers) {
			return fmt.Errorf("duplicate members in %s", x.ID)
		}
	}
	return nil
}

func validateSymbols(s Snapshot, ids validationIDs, modules map[string]Module, symbols map[string]Symbol) error {
	for _, x := range s.Symbols {
		if _, ok := modules[x.ModuleID]; !ok {
			return fmt.Errorf("unknown module %q", x.ModuleID)
		}
		prefix := "symbol:" + strings.TrimPrefix(x.ModuleID, "module:") + ":"
		raw, err := url.PathUnescape(strings.TrimPrefix(x.ID, prefix))
		if err != nil || raw == "" || !strings.HasPrefix(x.ID, prefix) || SymbolID(strings.TrimPrefix(x.ModuleID, "module:"), raw) != x.ID {
			return fmt.Errorf("invalid symbol ID %q", x.ID)
		}
		if x.Kind == "" || x.DisplaySignature == "" || (x.Visibility != "public" && x.Visibility != "private") {
			return fmt.Errorf("invalid symbol %s", x.ID)
		}
		if err := ids.add(x.ID, "symbol"); err != nil {
			return err
		}
		symbols[x.ID] = x
	}
	return nil
}

func validateContractMembers(s Snapshot, seams map[string]Seam, symbols map[string]Symbol) error {
	for _, m := range s.Modules {
		for _, id := range m.Seams {
			if _, ok := seams[id]; !ok {
				return fmt.Errorf("unknown seam %q", id)
			}
		}
	}
	for _, x := range s.Seams {
		for _, id := range x.ContractMembers {
			sym, ok := symbols[id]
			if !ok || sym.ModuleID != x.ModuleID || sym.Visibility != "public" {
				return fmt.Errorf("invalid contract member %q in %s", id, x.ID)
			}
		}
	}
	return nil
}

func validateRelations(s Snapshot, ids validationIDs) error {
	for _, r := range s.Relations {
		if !slices.Contains([]string{"imports", "uses", "implements", "exposes", "calls"}, r.Kind) {
			return fmt.Errorf("invalid relation kind %q", r.Kind)
		}
		if r.ID != RelationID(r.Kind, r.FromID, r.ToID) {
			return fmt.Errorf("noncanonical relation ID %q", r.ID)
		}
		if ids[r.FromID] == "" || ids[r.ToID] == "" || ids[r.FromID] == "relation" || ids[r.ToID] == "relation" {
			return fmt.Errorf("unknown relation endpoints in %s", r.ID)
		}
		if err := ids.add(r.ID, "relation"); err != nil {
			return err
		}
		if len(r.Evidence) == 0 {
			return fmt.Errorf("relation %s needs evidence", r.ID)
		}
		for _, e := range r.BypassEvidence {
			if !r.BypassesSeam || !slices.Contains(r.Evidence, e) {
				return fmt.Errorf("bypass evidence must belong to a bypass relation's evidence: %s", r.ID)
			}
		}
		for _, e := range r.Evidence {
			if err := validateEvidence(e); err != nil {
				return err
			}
			if e.Resolution != "resolved" {
				return fmt.Errorf("observed relation %s requires resolved evidence; emit a diagnostic for uncertainty", r.ID)
			}
		}
	}
	return nil
}

func validateDiagnostics(s Snapshot, ids validationIDs) error {
	for _, d := range s.Diagnostics {
		if d.Code == "" || d.Message == "" || !slices.Contains([]string{"error", "warning", "info"}, d.Severity) {
			return fmt.Errorf("invalid diagnostic %q", d.Code)
		}
		for _, id := range d.EntityIDs {
			if ids[id] == "" {
				return fmt.Errorf("unknown diagnostic entity %q", id)
			}
		}
		for _, e := range d.Evidence {
			if err := validateEvidence(e); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasDuplicate(ss []string) bool {
	seen := map[string]bool{}
	for _, s := range ss {
		if seen[s] {
			return true
		}
		seen[s] = true
	}
	return false
}
func sortedStrings(ss []string) []string {
	out := append([]string{}, ss...)
	slices.Sort(out)
	return slices.Compact(out)
}
func key(v any) string { b, _ := json.Marshal(v); return string(b) }
func evidence(es []Evidence) []Evidence {
	out := append([]Evidence{}, es...)
	slices.SortFunc(out, func(a, b Evidence) int { return strings.Compare(key(a), key(b)) })
	return slices.Compact(out)
}

// Canonical returns an independent, validated copy. Callers' slices are untouched.
func (s Snapshot) Canonical() (Snapshot, error) {
	if err := s.Validate(); err != nil {
		return Snapshot{}, err
	}
	out := Snapshot{SchemaVersion: s.SchemaVersion, Modules: []Module{}, Seams: []Seam{}, Symbols: append([]Symbol{}, s.Symbols...), Relations: []Relation{}, Diagnostics: []Diagnostic{}}
	for _, m := range s.Modules {
		m.SourceRoots = sortedStrings(m.SourceRoots)
		m.Seams = sortedStrings(m.Seams)
		out.Modules = append(out.Modules, m)
	}
	for _, x := range s.Seams {
		x.ContractMembers = sortedStrings(x.ContractMembers)
		out.Seams = append(out.Seams, x)
	}
	for _, r := range s.Relations {
		r.Evidence = evidence(r.Evidence)
		if r.BypassesSeam && len(r.BypassEvidence) == 0 {
			r.BypassEvidence = append([]Evidence{}, r.Evidence...)
		}
		r.BypassEvidence = evidence(r.BypassEvidence)
		out.Relations = append(out.Relations, r)
	}
	for _, d := range s.Diagnostics {
		d.EntityIDs = sortedStrings(d.EntityIDs)
		d.Evidence = evidence(d.Evidence)
		out.Diagnostics = append(out.Diagnostics, d)
	}
	slices.SortFunc(out.Modules, func(a, b Module) int { return strings.Compare(a.ID, b.ID) })
	slices.SortFunc(out.Seams, func(a, b Seam) int { return strings.Compare(a.ID, b.ID) })
	slices.SortFunc(out.Symbols, func(a, b Symbol) int { return strings.Compare(a.ID, b.ID) })
	slices.SortFunc(out.Relations, func(a, b Relation) int { return strings.Compare(a.ID, b.ID) })
	slices.SortFunc(out.Diagnostics, func(a, b Diagnostic) int { return strings.Compare(key(a), key(b)) })
	out.Diagnostics = slices.CompactFunc(out.Diagnostics, func(a, b Diagnostic) bool { return key(a) == key(b) })
	return out, nil
}
func (s Snapshot) JSON() ([]byte, error) {
	c, err := s.Canonical()
	if err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	return append(b, '\n'), err
}

// ArchitectureJSON is for architecture equivalence, never for evidence storage.
func (s Snapshot) ArchitectureJSON() ([]byte, error) {
	c, err := s.Canonical()
	if err != nil {
		return nil, err
	}
	c.Diagnostics = []Diagnostic{}
	for i := range c.Relations {
		c.Relations[i].Evidence = []Evidence{}
		c.Relations[i].BypassEvidence = nil
	}
	b, err := json.MarshalIndent(c, "", "  ")
	return append(b, '\n'), err
}

// Merge accepts duplicate identical entities, merges relation evidence, and
// rejects conflicting definitions rather than depending on adapter order.
func Merge(parts ...PartialSnapshot) (Snapshot, error) {
	s := Snapshot{SchemaVersion: SchemaVersion}
	seen := map[string]string{}
	relationIndex := map[string]int{}
	add := func(id string, v any) (bool, error) {
		k := key(v)
		if old, ok := seen[id]; ok {
			if old != k {
				return false, fmt.Errorf("conflicting definitions for %s", id)
			}
			return false, nil
		}
		seen[id] = k
		return true, nil
	}
	for _, p := range parts {
		if err := mergePartial(&s, p, add, relationIndex); err != nil {
			return s, err
		}
	}
	return s.Canonical()
}

// Decode rejects unsupported schema versions, unknown fields and trailing data.
func Decode(data []byte) (Snapshot, error) {
	var s Snapshot
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&s); err != nil {
		return s, fmt.Errorf("decode IR: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return s, fmt.Errorf("unexpected trailing IR data")
	}
	return s.Canonical()
}

// Owners maps module, seam and symbol IDs to declared module ownership.
func Owners(s Snapshot) map[string]string {
	owners := map[string]string{}
	for _, m := range s.Modules {
		owners[m.ID] = m.ID
	}
	for _, x := range s.Seams {
		owners[x.ID] = x.ModuleID
	}
	for _, x := range s.Symbols {
		owners[x.ID] = x.ModuleID
	}
	return owners
}

// IsConsumerRelation excludes declaration exposure and structural conformance:
// neither establishes an actual cross-module import/reference on its own.
func IsConsumerRelation(r Relation) bool {
	return r.Kind == "imports" || r.Kind == "uses" || r.Kind == "calls"
}

// Validate checks portable one-based source coordinates and resolution metadata.
func (e Evidence) Validate() error { return validateEvidence(e) }

// ValidRoot permits the project directory itself as a literal module root.
func ValidRoot(p string) bool { return p == "." || ValidPath(p) }

func mergePartial(s *Snapshot, p PartialSnapshot, add func(string, any) (bool, error), relationIndex map[string]int) error {
	for _, m := range p.Modules {
		m.SourceRoots = sortedStrings(m.SourceRoots)
		m.Seams = sortedStrings(m.Seams)
		ok, err := add(m.ID, m)
		if err != nil {
			return err
		}
		if ok {
			s.Modules = append(s.Modules, m)
		}
	}
	for _, x := range p.Seams {
		x.ContractMembers = sortedStrings(x.ContractMembers)
		ok, err := add(x.ID, x)
		if err != nil {
			return err
		}
		if ok {
			s.Seams = append(s.Seams, x)
		}
	}
	for _, x := range p.Symbols {
		ok, err := add(x.ID, x)
		if err != nil {
			return err
		}
		if ok {
			s.Symbols = append(s.Symbols, x)
		}
	}
	if err := mergeRelations(s, p.Relations, relationIndex); err != nil {
		return err
	}
	s.Diagnostics = append(s.Diagnostics, p.Diagnostics...)
	return nil
}

func mergeRelations(s *Snapshot, relations []Relation, relationIndex map[string]int) error {
	for _, r := range relations {
		if i, ok := relationIndex[r.ID]; ok {
			old := &s.Relations[i]
			if old.FromID != r.FromID || old.ToID != r.ToID || old.Kind != r.Kind {
				return fmt.Errorf("conflicting relation %s", r.ID)
			}
			if old.BypassesSeam && len(old.BypassEvidence) == 0 {
				old.BypassEvidence = append([]Evidence{}, old.Evidence...)
			}
			if r.BypassesSeam && len(r.BypassEvidence) == 0 {
				r.BypassEvidence = append([]Evidence{}, r.Evidence...)
			}
			old.Evidence = append(old.Evidence, r.Evidence...)
			old.BypassesSeam = old.BypassesSeam || r.BypassesSeam
			old.BypassEvidence = append(old.BypassEvidence, r.BypassEvidence...)
		} else {
			relationIndex[r.ID] = len(s.Relations)
			r.Evidence = append([]Evidence{}, r.Evidence...)
			r.BypassEvidence = append([]Evidence{}, r.BypassEvidence...)
			s.Relations = append(s.Relations, r)
		}
	}
	return nil
}
