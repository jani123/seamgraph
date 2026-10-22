package application

import (
	"context"
	"fmt"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"seamgraph/internal/snapshot"
	"seamgraph/internal/verification"
)

type Extract func(context.Context, string, string) (string, string, func(), error)
type Verify func(context.Context, string, ir.Snapshot, string) (verification.Report, error)
type Analyze func(context.Context, string) (ir.Snapshot, policy.Policy, error)
type Result struct {
	SchemaVersion        int            `json:"schemaVersion"`
	Base                 string         `json:"base"`
	Head                 string         `json:"head"`
	Diff                 snapshot.Diff  `json:"diff"`
	Before               ir.Snapshot    `json:"before"`
	After                ir.Snapshot    `json:"after"`
	Files                []FileChange   `json:"files"`
	Modules              []ModuleChange `json:"modules"`
	BeforeCheck          policy.Result  `json:"beforeCheck"`
	AfterCheck           policy.Result  `json:"afterCheck"`
	ConfigurationChanged bool           `json:"configurationChanged"`
}

func Compare(ctx context.Context, root, baseRef, headRef string, extract Extract, analyze Analyze, verify ...Verify) (Result, error) {
	return compare(ctx, root, baseRef, headRef, extract, analyze, nil, verify...)
}
func compare(ctx context.Context, root, baseRef, headRef string, extract Extract, analyze Analyze, cache *Cache, verify ...Verify) (Result, error) {
	result := Result{SchemaVersion: 1}
	base, commit, cleanup, err := extract(ctx, root, baseRef)
	if err != nil {
		return result, err
	}
	defer cleanup()
	result.Base = commit
	head, commit, cleanHead, err := extract(ctx, root, headRef)
	if err != nil {
		return result, err
	}
	defer cleanHead()
	result.Head = commit
	result.Files = []FileChange{}
	if cache != nil && cache.Inventory != nil {
		before, err := cache.Inventory(base)
		if err != nil {
			return result, fmt.Errorf("base files: %w", err)
		}
		after, err := cache.Inventory(head)
		if err != nil {
			return result, fmt.Errorf("head files: %w", err)
		}
		result.Files = fileChanges(before, after)
	}
	b, bp, err := cache.analyze(ctx, result.Base, base, analyze)
	if err != nil {
		return result, fmt.Errorf("base analysis: %w", err)
	}
	h, hp, err := cache.analyze(ctx, result.Head, head, analyze)
	if err != nil {
		return result, fmt.Errorf("head analysis: %w", err)
	}
	result.Diff, err = snapshot.ComparePolicies(b, h, bp, hp)
	if err != nil {
		return result, err
	}
	if len(verify) > 0 {
		before, err := verify[0](ctx, base, b, result.Base)
		if err != nil {
			return result, err
		}
		after, err := verify[0](ctx, head, h, result.Head)
		if err != nil {
			return result, err
		}
		result.Diff.Changes = append(result.Diff.Changes, snapshot.CompareVerification(before, after)...)
	}
	result.Before, result.After = b, h
	result.BeforeCheck, _ = policy.Evaluate(b, bp)
	result.AfterCheck, _ = policy.Evaluate(h, hp)
	for _, file := range result.Files {
		if file.Path == ConfigFilename || file.OldPath == ConfigFilename {
			result.ConfigurationChanged = true
		}
	}
	result.Modules = moduleChanges(b, h, result.Files, result.Diff.Changes)
	snapshot.SortChanges(result.Diff.Changes)
	return result, nil
}
