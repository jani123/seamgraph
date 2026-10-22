package application

import (
	"fmt"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"strings"
)

type Inspection struct {
	Entity    any
	Relations []ir.Relation
}

func Inspect(graph ir.Snapshot, result policy.Result, id string) (Inspection, error) {
	var entity any
	ids := []string{}
	for _, m := range graph.Modules {
		ids = append(ids, m.ID)
		if m.ID == id {
			entity = m
		}
	}
	for _, x := range graph.Seams {
		ids = append(ids, x.ID)
		if x.ID == id {
			entity = x
		}
	}
	for _, x := range graph.Symbols {
		ids = append(ids, x.ID)
		if x.ID == id {
			entity = x
		}
	}
	for _, r := range graph.Relations {
		ids = append(ids, r.ID)
		if r.ID == id {
			entity = r
		}
	}
	for _, v := range result.Violations {
		ids = append(ids, v.ID)
		if v.ID == id {
			entity = v
		}
	}
	if entity == nil {
		if len(ids) > 12 {
			ids = ids[:12]
		}
		return Inspection{}, fmt.Errorf("unknown ID %q; candidate IDs: %s", id, strings.Join(ids, ", "))
	}
	related := []ir.Relation{}
	owner := ir.Owners(graph)[id]
	members := map[string]bool{id: true}
	for _, seam := range graph.Seams {
		if seam.ID == id {
			for _, m := range seam.ContractMembers {
				members[m] = true
			}
		}
	}
	for _, r := range graph.Relations {
		if members[r.FromID] || members[r.ToID] || (strings.HasPrefix(id, "module:") && (ir.Owners(graph)[r.FromID] == owner || ir.Owners(graph)[r.ToID] == owner)) {
			related = append(related, r)
		}
	}
	return Inspection{entity, related}, nil
}
