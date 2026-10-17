// Package snapshot compares canonical IR architectures, without reading source
// files or checking out Git revisions.
package snapshot

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
)

type Change struct {
	Kind     string          `json:"kind"`
	EntityID string          `json:"entityId"`
	Status   string          `json:"status"`
	Before   json.RawMessage `json:"before,omitempty"`
	After    json.RawMessage `json:"after,omitempty"`
}
type Diff struct {
	SchemaVersion int      `json:"schemaVersion"`
	Changes       []Change `json:"changes"`
}

// Compare includes public contract and consumer changes, ignoring source
// locations, diagnostics and purely internal implementation changes.
func Compare(base, head ir.Snapshot) (Diff, error) {
	d := Diff{SchemaVersion: 1, Changes: []Change{}}
	b, err := architecture(base)
	if err != nil {
		return d, fmt.Errorf("base: %w", err)
	}
	h, err := architecture(head)
	if err != nil {
		return d, fmt.Errorf("head: %w", err)
	}
	for id, before := range b {
		after, ok := h[id]
		status := "modified"
		if !ok {
			status = "removed"
		}
		if !ok || string(before.value) != string(after.value) {
			d.Changes = append(d.Changes, Change{Kind: before.kind, EntityID: id, Status: status, Before: before.value, After: after.value})
		}
	}
	for id, after := range h {
		if _, ok := b[id]; !ok {
			d.Changes = append(d.Changes, Change{Kind: after.kind, EntityID: id, Status: "added", After: after.value})
		}
	}
	SortChanges(d.Changes)
	return d, nil
}

type entity struct {
	kind  string
	value json.RawMessage
}

func architecture(s ir.Snapshot) (map[string]entity, error) {
	c, err := s.Canonical()
	if err != nil {
		return nil, err
	}
	out := map[string]entity{}
	add := func(kind, id string, v any) { b, _ := json.Marshal(v); out[id] = entity{kind, b} }
	for _, m := range c.Modules {
		add("module", m.ID, m)
	}
	for _, x := range c.Seams {
		add("seam", x.ID, x)
	}
	relevant := map[string]bool{}
	for _, seam := range c.Seams {
		for _, id := range seam.ContractMembers {
			relevant[id] = true
		}
	}
	owners := ir.Owners(c)
	for _, r := range c.Relations {
		if ir.IsConsumerRelation(r) && owners[r.FromID] != owners[r.ToID] {
			relevant[r.ToID] = true
		}
	}
	for _, x := range c.Symbols {
		if relevant[x.ID] {
			add("symbol", x.ID, x)
		}
	}
	for _, r := range c.Relations {
		if !ir.IsConsumerRelation(r) || owners[r.FromID] == owners[r.ToID] {
			continue
		}
		// Retain private target identities on cross-boundary edges to show bypasses.
		r.Evidence = []ir.Evidence{}
		r.BypassEvidence = nil
		add("relation", r.ID, r)
	}
	return out, nil
}

// ComparePolicies adds regressions and resolutions by stable violation identity.
// Evaluation errors reject the diff rather than turning invalid input into clean.
func ComparePolicies(base, head ir.Snapshot, basePolicy, headPolicy policy.Policy) (Diff, error) {
	d, err := Compare(base, head)
	if err != nil {
		return d, err
	}
	b, err := policy.Evaluate(base, basePolicy)
	if err != nil {
		return d, err
	}
	h, err := policy.Evaluate(head, headPolicy)
	if err != nil {
		return d, err
	}
	if b.Incomplete || h.Incomplete {
		return d, fmt.Errorf("cannot compare policy regressions with incomplete analysis")
	}
	before := map[string]policy.Violation{}
	after := map[string]policy.Violation{}
	for _, v := range b.Violations {
		before[v.ID] = v
	}
	for _, v := range h.Violations {
		after[v.ID] = v
	}
	for id, v := range before {
		if _, ok := after[id]; !ok {
			v.Evidence = []ir.Evidence{}
			raw, _ := json.Marshal(v)
			d.Changes = append(d.Changes, Change{Kind: "violation", EntityID: id, Status: "removed", Before: raw})
		}
	}
	for id, v := range after {
		if _, ok := before[id]; !ok {
			v.Evidence = []ir.Evidence{}
			raw, _ := json.Marshal(v)
			d.Changes = append(d.Changes, Change{Kind: "violation", EntityID: id, Status: "added", After: raw})
		}
	}
	SortChanges(d.Changes)
	return d, nil
}
func SortChanges(changes []Change) {
	slices.SortFunc(changes, func(a, b Change) int {
		if c := strings.Compare(a.Kind, b.Kind); c != 0 {
			return c
		}
		return strings.Compare(a.EntityID, b.EntityID)
	})
}
func (d Diff) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(d, "", "  ")
	return append(b, '\n'), err
}
