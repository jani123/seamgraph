package application

import (
	"fmt"
	"strings"
)

// ReviewMarkdown includes the selected activity view and complete original facts.
func ReviewMarkdown(r ReviewWorkspace) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# Local code review\n\nComparison: %s → %s (two trees, not a simulated merge).\n\n", markdownLiteral(r.Comparison.Base), markdownLiteral(r.Comparison.Head))
	if r.ArchitectureIssue != "" {
		fmt.Fprintf(&out, "%s\n\n", markdownLiteral(r.ArchitectureIssue))
	}
	if r.Range != nil {
		fmt.Fprintf(&out, "Mode: %s. Target: %s (%s). Review: %s (%s).\n\n", markdownLiteral(r.Range.Mode), markdownLiteral(r.Range.Target), markdownLiteral(r.Range.TargetRevision), markdownLiteral(r.Range.Review), markdownLiteral(r.Range.Head))
	}
	fmt.Fprintf(&out, "History range: %s..%s. %s.\n\n", markdownLiteral(r.Activity.Base), markdownLiteral(r.Activity.Head), r.Activity.Policy)
	fmt.Fprintf(&out, "Filters: paths %s; type %s; scope %s; exclusions %s; group %s; metric %s; sort %s; touching file %s; selected group %s.\n\n", markdownLiteral(r.PathFilter), markdownLiteral(r.TypeFilter), markdownLiteral(r.Options["commitScope"]), markdownLiteral(r.Options["exclude"]), markdownLiteral(r.Options["group"]), markdownLiteral(r.Options["metric"]), markdownLiteral(r.Options["sort"]), markdownLiteral(r.Options["historyFile"]), markdownLiteral(r.Options["historyGroup"]))
	out.WriteString("Frequency counts distinct touching commits. Churn is additions plus deletions, including reverted work. Net +/− compares final trees. Binary line statistics are unknown. Paths are not followed across renames; edited moves are uncertain. Merges are excluded; squash history hides original edits. Activity is review guidance, not proof of defects, complexity or productivity. Type/scope filters select commits and paths; hotspot totals retain the full history range for those paths.\n\n## Hotspots\n\n")
	for _, h := range r.Summary.Hotspots {
		fmt.Fprintf(&out, "- %s: %d commits", markdownLiteral(h.Path), h.Touches)
		if h.Binary {
			out.WriteString("; added/deleted/churn unknown (binary)")
		} else {
			fmt.Fprintf(&out, "; +%d / -%d; churn %d", h.Added, h.Deleted, h.Churn)
		}
		if h.Net.Binary {
			out.WriteString("; net unknown (binary)\n")
		} else {
			fmt.Fprintf(&out, "; net +%d / -%d\n", h.Net.Added, h.Net.Deleted)
		}
	}
	out.WriteString("\n## Commit counts\n\n")
	for _, group := range r.Summary.Types {
		fmt.Fprintf(&out, "- Type %s: %d commits\n", markdownLiteral(group.Name), group.Count)
	}
	for _, group := range r.Summary.Scopes {
		fmt.Fprintf(&out, "- Scope %s: %d commits\n", markdownLiteral(group.Name), group.Count)
	}
	out.WriteString("\n## Complete evidence\n\nFull source includes only the selected file. Saved findings retain bounded original source excerpts. The JSON includes all raw commits and metrics as well as the selected summary.\n\n```json\n")
	out.WriteString(ReviewJSON(r))
	out.WriteString("\n```\n")
	return out.String()
}
