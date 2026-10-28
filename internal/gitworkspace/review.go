package gitworkspace

import (
	"context"
	"fmt"
	"seamgraph/internal/ir"
	"strings"
)

// BranchRange resolves refs without checking out code or choosing an arbitrary
// merge base when criss-cross history supplies several equally valid bases.
func BranchRange(ctx context.Context, root, target, review, mode string) (ir.ReviewRange, error) {
	r := ir.ReviewRange{Mode: mode, Target: target, Review: review}
	if mode != "merge-base" && mode != "target-tip" {
		return r, fmt.Errorf("unknown branch comparison mode")
	}
	var err error
	r.TargetRevision, err = Revision(ctx, root, target)
	if err != nil {
		return r, fmt.Errorf("target revision %q is missing or unavailable: %w", target, err)
	}
	r.Head, err = Revision(ctx, root, review)
	if err != nil {
		return r, fmt.Errorf("review revision %q is missing or unavailable: %w", review, err)
	}
	shallow, err := git(ctx, root, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return r, err
	}
	if strings.TrimSpace(string(shallow)) == "true" {
		return r, fmt.Errorf("branch history is shallow; fetch complete history before reviewing this range")
	}
	r.Base = r.TargetRevision
	if mode == "target-tip" {
		return r, nil
	}
	data, err := git(ctx, root, "merge-base", "--all", r.TargetRevision, r.Head)
	if err != nil {
		return r, fmt.Errorf("no merge base is available; fetch missing history or choose target-tip comparison: %w", err)
	}
	bases := strings.Fields(string(data))
	if len(bases) != 1 {
		return r, fmt.Errorf("ambiguous merge bases (%d); choose target-tip comparison or explicit revisions", len(bases))
	}
	r.Base = bases[0]
	return r, nil
}
