package application

import (
	"context"
	"sync"

	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
)

type cached struct {
	snapshot ir.Snapshot
	policy   policy.Policy
}

// Cache is bounded to a server session; context identifies its analysis settings.
// No cache files or Git worktrees are left in the user's repository.
type Cache struct {
	Context   string
	ConfigKey func(string) (string, error)
	Inventory func(string) (map[string]string, error)
	mu        sync.Mutex
	entries   map[string]cached
}

func (c *Cache) analyze(ctx context.Context, commit, root string, analyze Analyze) (ir.Snapshot, policy.Policy, error) {
	if c == nil {
		return analyze(ctx, root)
	}
	key := commit + ":" + c.Context
	if c.ConfigKey != nil {
		digest, err := c.ConfigKey(root)
		if err != nil {
			return ir.Snapshot{}, policy.Policy{}, err
		}
		key += ":" + digest
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.entries[key]; ok {
		return v.snapshot, v.policy, nil
	}
	s, p, err := analyze(ctx, root)
	if err != nil {
		return s, p, err
	}
	r, err := policy.Evaluate(s, p)
	if err != nil {
		return s, p, err
	}
	if !r.Incomplete {
		if len(c.entries) >= 64 || c.entries == nil {
			c.entries = map[string]cached{}
		}
		c.entries[key] = cached{s, p}
	}
	return s, p, nil
}
func (c *Cache) Compare(ctx context.Context, root, base, head string, extract Extract, analyze Analyze, verify ...Verify) (Result, error) {
	return compare(ctx, root, base, head, extract, analyze, c, verify...)
}
