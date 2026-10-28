package application

import (
	"path"
	"seamgraph/internal/ir"
	"slices"
	"strings"
)

type CommitCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
type ReviewSummary struct {
	Hotspots   []Hotspot         `json:"hotspots"`
	Types      []CommitCount     `json:"types"`
	Scopes     []CommitCount     `json:"scopes"`
	Commits    []ir.ReviewCommit `json:"commits"`
	Metric     string            `json:"metric"`
	Group      string            `json:"group"`
	Exclusions string            `json:"exclusions"`
}

// SummarizeReview retains full-range totals for matching paths, while commit
// filters select the displayed history. Exports retain both this view and raw facts.
func SummarizeReview(r ReviewWorkspace) ReviewSummary {
	result := ReviewSummary{Metric: r.Options["metric"], Group: r.Options["group"], Exclusions: r.Options["exclude"]}
	available := reviewCommits(r)
	result.Types = commitCounts(available, false)
	result.Scopes = commitCounts(available, true)
	for _, c := range available {
		if matchesCommit(c, r.TypeFilter, r.Options["commitScope"]) {
			result.Commits = append(result.Commits, c)
		}
	}
	rows := []Hotspot{}
	for _, h := range r.Hotspots {
		if !reviewPathVisible(h.Path, r) || !touchesFiltered(h.Path, result.Commits) {
			continue
		}
		rows = append(rows, h)
	}
	result.Hotspots = groupHotspots(r, rows)
	sortHotspots(result.Hotspots, result.Metric, r.Options["sort"])
	return result
}
func reviewCommits(r ReviewWorkspace) []ir.ReviewCommit {
	result := []ir.ReviewCommit{}
	for _, c := range r.Activity.Commits {
		for file := range c.Files {
			if reviewPathVisible(file, r) {
				result = append(result, c)
				break
			}
		}
	}
	return result
}
func reviewPathVisible(file string, r ReviewWorkspace) bool {
	if !strings.Contains(strings.ToLower(file), strings.ToLower(r.PathFilter)) {
		return false
	}
	if r.Options["historyFile"] != "" && file != r.Options["historyFile"] {
		return false
	}
	if group := r.Options["historyGroup"]; group != "" && hotspotGroup(r, file) != group {
		return false
	}
	exclusion := r.Options["exclude"]
	if strings.Contains(exclusion, "vendor") && (strings.HasPrefix(file, "vendor/") || strings.Contains("/"+file, "/node_modules/") || strings.Contains("/"+file, "/vendor/")) {
		return false
	}
	generated := strings.Contains("/"+file, "/generated/") || strings.Contains(file, ".generated.") || strings.Contains(file, "_generated.") || strings.HasSuffix(file, ".pb.go")
	return !strings.Contains(exclusion, "generated") || !generated
}
func matchesCommit(c ir.ReviewCommit, kind, scope string) bool {
	if kind != "" && c.Type != kind {
		return false
	}
	if scope == "(none)" {
		return c.Scope == ""
	}
	return scope == "" || c.Scope == scope
}
func touchesFiltered(file string, commits []ir.ReviewCommit) bool {
	for _, c := range commits {
		if _, ok := c.Files[file]; ok {
			return true
		}
	}
	return false
}
func commitCounts(commits []ir.ReviewCommit, scope bool) []CommitCount {
	counts := map[string]int{}
	for _, c := range commits {
		name := c.Type
		if scope {
			name = c.Scope
			if name == "" {
				name = "(none)"
			}
		}
		counts[name]++
	}
	result := []CommitCount{}
	for name, count := range counts {
		result = append(result, CommitCount{name, count})
	}
	slices.SortFunc(result, func(a, b CommitCount) int { return strings.Compare(a.Name, b.Name) })
	return result
}
func hotspotGroup(r ReviewWorkspace, file string) string {
	switch r.Options["group"] {
	case "directory":
		return path.Dir(file)
	case "module":
		for _, f := range r.Code.Files {
			if f.Path == file && f.Module != "" {
				return f.Module
			}
		}
		return "module:unassigned"
	default:
		return file
	}
}
func groupHotspots(r ReviewWorkspace, rows []Hotspot) []Hotspot {
	groups := map[string]Hotspot{}
	for _, h := range rows {
		name := hotspotGroup(r, h.Path)
		g := groups[name]
		g.Path = name
		g.Paths = append(g.Paths, h.Path)
		g.Added += h.Added
		g.Deleted += h.Deleted
		g.Churn += h.Churn
		g.Binary = g.Binary || h.Binary
		g.Net.Added += h.Net.Added
		g.Net.Deleted += h.Net.Deleted
		g.Net.Binary = g.Net.Binary || h.Net.Binary
		groups[name] = g
	}
	result := []Hotspot{}
	for _, g := range groups {
		g.Touches = groupTouches(g.Paths, r.Activity.Commits)
		result = append(result, g)
	}
	return result
}
func groupTouches(paths []string, commits []ir.ReviewCommit) int {
	count := 0
	for _, c := range commits {
		for _, file := range paths {
			if _, ok := c.Files[file]; ok {
				count++
				break
			}
		}
	}
	return count
}
func hotspotMetric(h Hotspot, metric string) int {
	switch metric {
	case "churn":
		return h.Churn
	case "net":
		return h.Net.Added + h.Net.Deleted
	default:
		return h.Touches
	}
}
func sortHotspots(rows []Hotspot, metric, order string) {
	slices.SortFunc(rows, func(a, b Hotspot) int {
		if order == "path" {
			return strings.Compare(a.Path, b.Path)
		}
		delta := hotspotMetric(b, metric) - hotspotMetric(a, metric)
		if unknownMetric(a, metric) != unknownMetric(b, metric) {
			if unknownMetric(a, metric) {
				return 1
			}
			return -1
		}
		if order == "ascending" {
			delta = -delta
		}
		if delta != 0 {
			return delta
		}
		return strings.Compare(a.Path, b.Path)
	})
}

func unknownMetric(h Hotspot, metric string) bool {
	if metric == "net" {
		return h.Net.Binary
	}
	return metric == "churn" && h.Binary
}
