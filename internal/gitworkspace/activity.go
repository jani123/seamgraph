package gitworkspace

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"seamgraph/internal/ir"
)

var commitSubject = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9-]*)(?:\(([^()\r\n]+)\))?(!)?: .+`)

// Activity excludes merges and counts paths without rename following. Git's
// complete numstat records keep binary/unavailable line counts explicit.
func Activity(ctx context.Context, root, base, head, file string) (ir.Activity, error) {
	a := ir.Activity{Base: base, Head: head, Policy: "base..head; merges excluded; no rename following; churn = additions + deletions", Commits: []ir.ReviewCommit{}, Net: map[string]ir.LineStats{}}
	if strings.HasPrefix(head, "WORKTREE:") || strings.HasPrefix(base, "WORKTREE:") {
		a.Unavailable = "Commit activity is unavailable for a working-tree comparison"
		return a, nil
	}
	if file != "" && !ir.ValidPath(file) {
		return a, fmt.Errorf("invalid activity path")
	}
	listing, err := git(ctx, root, "rev-list", "--no-merges", "--max-count=501", base+".."+head, "--")
	if err != nil {
		return a, err
	}
	revisions := strings.Fields(string(listing))
	if len(revisions) > 500 {
		return a, fmt.Errorf("review history exceeds 500 commits; choose a narrower range")
	}
	for _, revision := range revisions {
		commit, err := activityCommit(ctx, root, revision, file)
		if err != nil {
			return a, err
		}
		a.Commits = append(a.Commits, commit)
	}
	data, err := git(ctx, root, "diff", "--no-ext-diff", "--no-textconv", "--numstat", "-z", "--no-renames", base, head, "--")
	if err != nil {
		return a, err
	}
	a.Net, err = parseNumstat(data)
	if err != nil {
		return a, err
	}
	if file != "" {
		a.Patch, err = activityPatch(ctx, root, []string{"diff", "--no-ext-diff", "--no-textconv", "--no-renames", base, head, "--", file})
	}
	return a, err
}

func activityCommit(ctx context.Context, root, revision, file string) (ir.ReviewCommit, error) {
	c := ir.ReviewCommit{Revision: revision}
	message, err := git(ctx, root, "show", "-s", "--format=%B", revision, "--")
	if err != nil {
		return c, err
	}
	// Git's format adds one newline after the raw commit message.
	c.Message = strings.TrimSuffix(string(message), "\n")
	c.Type, c.Scope, c.Breaking = classifyCommit(c.Message)
	data, err := git(ctx, root, "diff-tree", "--no-ext-diff", "--no-textconv", "--root", "--no-commit-id", "--numstat", "-z", "-r", "--no-renames", revision, "--")
	if err != nil {
		return c, err
	}
	c.Files, err = parseNumstat(data)
	if err != nil {
		return c, err
	}
	if _, ok := c.Files[file]; file != "" && ok {
		c.Patch, err = activityPatch(ctx, root, []string{"show", "--format=", "--no-ext-diff", "--no-textconv", "--no-renames", revision, "--", file})
	}
	return c, err
}

func classifyCommit(message string) (kind, scope string, breaking bool) {
	kind = "unmatched"
	if parts := commitSubject.FindStringSubmatch(strings.SplitN(message, "\n", 2)[0]); parts != nil {
		kind, scope, breaking = parts[1], parts[2], parts[3] != ""
	}
	for _, line := range strings.Split(message, "\n") {
		if strings.HasPrefix(line, "BREAKING CHANGE: ") || strings.HasPrefix(line, "BREAKING-CHANGE: ") {
			breaking = true
		}
	}
	return kind, scope, breaking
}

func parseNumstat(data []byte) (map[string]ir.LineStats, error) {
	result := map[string]ir.LineStats{}
	for _, record := range bytes.Split(data, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		fields := strings.SplitN(string(record), "\t", 3)
		if len(fields) != 3 || !ir.ValidPath(fields[2]) {
			return nil, fmt.Errorf("invalid Git numstat record")
		}
		stats := ir.LineStats{}
		if fields[0] == "-" && fields[1] == "-" {
			stats.Binary = true
		} else {
			added, err := strconv.Atoi(fields[0])
			if err != nil || added < 0 {
				return nil, fmt.Errorf("invalid Git additions")
			}
			deleted, err := strconv.Atoi(fields[1])
			if err != nil || deleted < 0 {
				return nil, fmt.Errorf("invalid Git deletions")
			}
			stats.Added = added
			stats.Deleted = deleted
		}
		result[fields[2]] = stats
	}
	return result, nil
}

func activityPatch(ctx context.Context, root string, args []string) (string, error) {
	data, err := git(ctx, root, args...)
	if err != nil {
		return "", err
	}
	if len(data) > 2*1024*1024 {
		return "Patch exceeds the 2 MiB display limit; use Git locally.", nil
	}
	return string(data), nil
}
