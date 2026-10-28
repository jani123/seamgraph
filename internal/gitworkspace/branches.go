package gitworkspace

import (
	"context"
	"seamgraph/internal/ir"
	"strings"
)

// Branches reads refs already present in the repository, without checkout,
// network access or assuming that a remote's default branch is named main.
func Branches(ctx context.Context, root string) (ir.BranchChoices, error) {
	choices := ir.BranchChoices{Items: []ir.BranchChoice{}}
	data, err := git(ctx, root, "for-each-ref", "--sort=refname", "--format=%(refname)%00%(symref)", "refs/heads/", "refs/remotes/")
	if err != nil {
		return choices, err
	}
	current, _ := git(ctx, root, "symbolic-ref", "--quiet", "HEAD")
	choices.Current = strings.TrimPrefix(strings.TrimSpace(string(current)), "refs/heads/")
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		ref, symbolic, ok := strings.Cut(line, "\x00")
		if !ok || symbolic != "" {
			continue
		}
		choice := branchChoice(ref, choices.Current)
		choices.Items = append(choices.Items, choice)
	}
	choices.Default = defaultBranch(ctx, root, choices.Items)
	return choices, nil
}
func branchChoice(ref, current string) ir.BranchChoice {
	if name, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
		return ir.BranchChoice{Ref: name, Name: name, Current: name == current}
	}
	return ir.BranchChoice{Ref: ref, Name: strings.TrimPrefix(ref, "refs/remotes/"), Remote: true}
}
func defaultBranch(ctx context.Context, root string, items []ir.BranchChoice) string {
	data, err := git(ctx, root, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
	if err == nil {
		ref := strings.TrimSpace(string(data))
		local := strings.TrimPrefix(ref, "refs/remotes/origin/")
		for _, item := range items {
			if !item.Remote && item.Ref == local {
				return local
			}
		}
		for _, item := range items {
			if item.Ref == ref {
				return ref
			}
		}
	}
	for _, name := range []string{"main", "master"} {
		for _, item := range items {
			if item.Ref == name {
				return name
			}
		}
	}
	return ""
}
