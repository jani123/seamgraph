package ui

import (
	"fmt"
	"strings"

	"seamgraph/internal/application"
	"seamgraph/internal/ir"
)

type consoleEntry struct {
	Severity, Code, Message string
	Evidence                []ir.Evidence
	EntityIDs               []string
}

func codeDiagnostics(browser application.CodeBrowser, filter CodeFilter) []consoleEntry {
	entries := []consoleEntry{}
	seen := map[string]bool{}
	add := func(entry consoleEntry) {
		key := fmt.Sprintf("%s:%s:%v:%v", entry.Severity, entry.Message, entry.Evidence, entry.EntityIDs)
		if !seen[key] {
			entries = append(entries, entry)
			seen[key] = true
		}
	}
	for _, issue := range browser.Issues {
		add(consoleEntry{Severity: "error", Code: "analysis", Message: issue})
	}
	for _, d := range browser.Diagnostics {
		add(consoleEntry{Severity: d.Severity, Code: d.Code, Message: d.Message, Evidence: d.Evidence, EntityIDs: d.EntityIDs})
	}
	if filter.ReviewIssue != "" {
		add(consoleEntry{Severity: "warning", Code: "review-context", Message: "Optional architecture and saved-review context is unavailable: " + filter.ReviewIssue})
	}
	return entries
}

func consoleTotal(entries []consoleEntry, severity string) int {
	count := 0
	for _, entry := range entries {
		if entry.Severity == severity {
			count++
		}
	}
	return count
}

func consoleCount(entries []consoleEntry, severity string) string {
	count := consoleTotal(entries, severity)
	label := severity
	if count != 1 {
		label += "s"
	}
	return fmt.Sprintf("%d %s", count, label)
}

func consoleLabel(entries []consoleEntry) string {
	return "Diagnostics console: " + consoleCount(entries, "error") + ", " + consoleCount(entries, "warning")
}

func consoleEntities(entry consoleEntry) string { return strings.Join(entry.EntityIDs, ", ") }
