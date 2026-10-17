package snapshot

import (
	"encoding/json"
	"seamgraph/internal/verification"
)

// CompareVerification compares declared behavior and verification status, never
// raw run timestamps. Exact source evidence remains in the original reports.
func CompareVerification(base, head verification.Report) []Change {
	changes := []Change{}
	b := map[string]entity{}
	h := map[string]entity{}
	index := func(report verification.Report, out map[string]entity) {
		for _, r := range report.Requirements {
			raw, _ := json.Marshal(r)
			out[r.ID] = entity{"requirement", raw}
		}
		for _, r := range report.Results {
			value := struct {
				SeamID       string                `json:"seamId"`
				Status       string                `json:"status"`
				Members      verification.Coverage `json:"memberExecutionCoverage"`
				Requirements verification.Coverage `json:"declaredRequirementVerification"`
			}{r.SeamID, r.Status, r.MemberCoverage, r.RequirementCoverage}
			raw, _ := json.Marshal(value)
			out["verification:"+r.SeamID] = entity{"verification", raw}
		}
	}
	index(base, b)
	index(head, h)
	for id, before := range b {
		after, ok := h[id]
		if !ok || string(before.value) != string(after.value) {
			status := "modified"
			if !ok {
				status = "removed"
			}
			changes = append(changes, Change{Kind: before.kind, EntityID: id, Status: status, Before: before.value, After: after.value})
		}
	}
	for id, after := range h {
		if _, ok := b[id]; !ok {
			changes = append(changes, Change{Kind: after.kind, EntityID: id, Status: "added", After: after.value})
		}
	}
	SortChanges(changes)
	return changes
}
