package application

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"seamgraph/internal/snapshot"
)

type FileChange struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"`
	Status  string `json:"status"`
}
type ModuleChange struct {
	ID                string            `json:"id"`
	Status            string            `json:"status,omitempty"`
	Files             []FileChange      `json:"files"`
	Seams             []snapshot.Change `json:"seams"`
	PolicyChanged     bool              `json:"policyChanged,omitempty"`
	DefinitionChanged bool              `json:"definitionChanged,omitempty"`
}

// fileChanges pairs exact-content moves deterministically. Edited moves remain
// an addition and deletion; both module owners are still marked correctly.
func fileChanges(before, after map[string]string) []FileChange {
	changes := []FileChange{}
	removed := map[string][]string{}
	for path, digest := range before {
		if next, ok := after[path]; !ok {
			removed[digest] = append(removed[digest], path)
		} else if next != digest {
			changes = append(changes, FileChange{Path: path, Status: "modified"})
		}
	}
	for digest := range removed {
		sort.Strings(removed[digest])
	}
	added := []string{}
	for path := range after {
		if _, ok := before[path]; !ok {
			added = append(added, path)
		}
	}
	sort.Strings(added)
	for _, path := range added {
		old := removed[after[path]]
		if len(old) > 0 {
			changes = append(changes, FileChange{Path: path, OldPath: old[0], Status: "renamed"})
			removed[after[path]] = old[1:]
		} else {
			changes = append(changes, FileChange{Path: path, Status: "added"})
		}
	}
	for _, paths := range removed {
		for _, path := range paths {
			changes = append(changes, FileChange{Path: path, Status: "removed"})
		}
	}
	slices.SortFunc(changes, func(a, b FileChange) int { return strings.Compare(a.Path, b.Path) })
	return changes
}
func pathOwner(graph ir.Snapshot, file string) string {
	for _, m := range graph.Modules {
		for _, root := range m.SourceRoots {
			if root == "." || file == root || strings.HasPrefix(file, root+"/") {
				return m.ID
			}
		}
	}
	return ""
}
func moduleChanges(before, after ir.Snapshot, files []FileChange, architecture []snapshot.Change) []ModuleChange {
	changes := map[string]*ModuleChange{}
	get := func(id string) *ModuleChange {
		if changes[id] == nil {
			changes[id] = &ModuleChange{ID: id, Files: []FileChange{}, Seams: []snapshot.Change{}}
		}
		return changes[id]
	}
	bm, hm := map[string]bool{}, map[string]bool{}
	for _, m := range before.Modules {
		bm[m.ID] = true
	}
	for _, m := range after.Modules {
		hm[m.ID] = true
	}
	for id := range bm {
		if !hm[id] {
			get(id).Status = "removed"
		}
	}
	for id := range hm {
		if !bm[id] {
			get(id).Status = "added"
		}
	}
	for _, file := range files {
		// The rule document is configuration, even when a module owns the root.
		if file.Path == ConfigFilename || file.OldPath == ConfigFilename {
			continue
		}
		old := file.Path
		if file.OldPath != "" {
			old = file.OldPath
		}
		owners := map[string]bool{}
		if file.Status != "added" {
			owners[pathOwner(before, old)] = true
		}
		if file.Status != "removed" {
			owners[pathOwner(after, file.Path)] = true
		}
		for id := range owners {
			if id != "" {
				get(id).Files = append(get(id).Files, file)
			}
		}
	}
	owners := ir.Owners(before)
	for id, owner := range ir.Owners(after) {
		owners[id] = owner
	}
	members := map[string]bool{}
	for _, g := range []ir.Snapshot{before, after} {
		for _, seam := range g.Seams {
			for _, id := range seam.ContractMembers {
				members[id] = true
			}
		}
	}
	for _, change := range architecture {
		switch change.Kind {
		case "violation":
			raw := change.After
			if len(raw) == 0 {
				raw = change.Before
			}
			var violation policy.Violation
			if json.Unmarshal(raw, &violation) == nil {
				for _, id := range []string{violation.ConsumerID, violation.ProviderID} {
					if owner := owners[id]; owner != "" {
						get(owner).PolicyChanged = true
					}
				}
			}
		case "module":
			if change.Status == "modified" {
				get(change.EntityID).DefinitionChanged = true
			}
		case "seam", "symbol":
			if change.Kind == "symbol" && !members[change.EntityID] {
				continue
			}
			if id := owners[change.EntityID]; id != "" {
				get(id).Seams = append(get(id).Seams, change)
			}
		}
	}
	result := []ModuleChange{}
	for _, change := range changes {
		if change.Status == "" {
			change.Status = "modified"
		}
		result = append(result, *change)
	}
	slices.SortFunc(result, func(a, b ModuleChange) int { return strings.Compare(a.ID, b.ID) })
	return result
}
