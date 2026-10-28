package ui

import (
	"seamgraph/internal/ir"
	"strings"
)

func isBranchComparison(base string) bool {
	return strings.HasPrefix(base, "merge-base:") || strings.HasPrefix(base, "target-tip:")
}
func branchSelection(choices ir.BranchChoices, value string, target bool, active bool) string {
	if active && value != "" && value != "WORKTREE" {
		return value
	}
	if target {
		return choices.Default
	}
	return choices.Current
}
func knownBranch(choices ir.BranchChoices, value string) bool {
	for _, item := range choices.Items {
		if item.Ref == value {
			return true
		}
	}
	return value == ""
}
func branchLabel(item ir.BranchChoice) string {
	label := item.Name
	if item.Current {
		label += " (current)"
	}
	return label
}
