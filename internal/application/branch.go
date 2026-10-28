package application

import (
	"context"
	"seamgraph/internal/ir"
	"strings"
)

// Prefixes are comparison identities, retained in review state and links. Only
// the resolved commit hashes are ever passed to tree extraction or analysis.
func (s *Service) reviewRange(ctx context.Context, base, head string) (*ir.ReviewRange, error) {
	for _, mode := range []string{"merge-base", "target-tip"} {
		if target, ok := strings.CutPrefix(base, mode+":"); ok {
			r, err := s.ports.BranchRange(ctx, s.root, target, head, mode)
			return &r, err
		}
	}
	return nil, nil
}

func (s *Service) Branches(ctx context.Context) ir.BranchChoices {
	if s.ports.Branches == nil {
		return ir.BranchChoices{Unavailable: "Branch selection is unavailable"}
	}
	choices, err := s.ports.Branches(ctx, s.root)
	if err != nil {
		choices.Unavailable = "Branch selection needs an available Git repository"
	}
	return choices
}
