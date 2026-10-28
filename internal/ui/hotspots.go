package ui

import (
	"fmt"
	"net/url"
	"seamgraph/internal/application"
	"seamgraph/internal/ir"
	"slices"
	"strings"
	"time"
)

func activityLink(r application.ReviewWorkspace, key, value, tab string) string {
	address, _ := url.Parse(reviewLink(r, r.Selected, "", "", r.TypeFilter))
	q := address.Query()
	q.Set(key, value)
	q.Set("tab", tab)
	address.RawQuery = q.Encode()
	return address.String()
}
func hotspotDestination(r application.ReviewWorkspace, h application.Hotspot) string {
	if len(h.Paths) > 1 {
		return activityLink(r, "historyGroup", h.Path, "history")
	}
	return activityLink(r, "historyFile", h.Paths[0], "history")
}
func hotspotSource(r application.ReviewWorkspace, h application.Hotspot) string {
	file := h.Path
	if len(h.Paths) > 0 {
		file = h.Paths[0]
	}
	for _, f := range r.Code.Files {
		if f.Path == file {
			return reviewLink(r, file, "", "", r.TypeFilter)
		}
	}
	for _, c := range r.Activity.Commits {
		if _, ok := c.Files[file]; ok {
			return historySourceLink(c, file)
		}
	}
	return ""
}
func heatStyle(h application.Hotspot, rows []application.Hotspot, metric string) string {
	maxValue := 1
	for _, row := range rows {
		maxValue = max(maxValue, heatValue(row, metric))
	}
	return fmt.Sprintf("background:rgba(5,150,105,%.3f)", .15+.85*float64(heatValue(h, metric))/float64(maxValue))
}
func heatValue(h application.Hotspot, metric string) int {
	if heatUnknown(h, metric) {
		return 0
	}
	switch metric {
	case "churn":
		return h.Churn
	case "net":
		return h.Net.Added + h.Net.Deleted
	default:
		return h.Touches
	}
}
func commitFiles(c ir.ReviewCommit, r application.ReviewWorkspace) []string {
	paths := []string{}
	for file := range c.Files {
		paths = append(paths, file)
	}
	slices.SortFunc(paths, func(a, b string) int {
		if result := strings.Compare(reviewFileGroup(r, a), reviewFileGroup(r, b)); result != 0 {
			return result
		}
		if result := strings.Compare(reviewFileSeams(r, a), reviewFileSeams(r, b)); result != 0 {
			return result
		}
		return strings.Compare(a, b)
	})
	return paths
}
func timelineTypes(r application.ReviewWorkspace) []string {
	names := []string{}
	for _, c := range r.Summary.Commits {
		if !slices.Contains(names, c.Type) {
			names = append(names, c.Type)
		}
	}
	slices.Sort(names)
	return names
}

func heatUnknown(h application.Hotspot, metric string) bool {
	if metric == "net" {
		return h.Net.Binary
	}
	return metric == "churn" && h.Binary
}
func timelineCommits(r application.ReviewWorkspace, kind string) []ir.ReviewCommit {
	commits := []ir.ReviewCommit{}
	for _, c := range r.Summary.Commits {
		if c.Type == kind {
			commits = append(commits, c)
		}
	}
	slices.SortFunc(commits, func(a, b ir.ReviewCommit) int {
		left, _ := time.Parse(time.RFC3339, a.Date)
		right, _ := time.Parse(time.RFC3339, b.Date)
		if order := left.Compare(right); order != 0 {
			return order
		}
		return strings.Compare(a.Revision, b.Revision)
	})
	return commits
}
