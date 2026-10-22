package application

import (
	"context"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"seamgraph/internal/verification"
	"sync"
)

// InspectionSession owns any temporary revision tree. Close is idempotent; the
// caller must close even when it only consumes the architecture snapshot.
type InspectionSession struct {
	Snapshot ir.Snapshot
	Policy   policy.Policy
	Check    policy.Result
	verify   func(context.Context) (verification.Report, error)
	cleanup  func()
	once     sync.Once
}

func (s *InspectionSession) Close() { s.once.Do(s.cleanup) }
func (s *InspectionSession) Verify(ctx context.Context) (verification.Report, error) {
	return s.verify(ctx)
}
func (s *Service) InspectProject(ctx context.Context, root, ref string) (*InspectionSession, error) {
	storeRoot := root
	commit := ""
	cleanup := func() {}
	if ref != "" {
		var err error
		root, commit, cleanup, err = s.ports.Extract(ctx, root, ref)
		if err != nil {
			return nil, err
		}
	}
	graph, p, err := s.analyze(ctx, root)
	if err != nil {
		cleanup()
		return nil, err
	}
	result, err := s.check(graph, p)
	if err != nil {
		cleanup()
		return nil, err
	}
	return &InspectionSession{Snapshot: graph, Policy: p, Check: result, cleanup: cleanup, verify: func(ctx context.Context) (verification.Report, error) {
		if commit != "" {
			return s.revisionContracts(storeRoot)(ctx, root, graph, commit)
		}
		return s.contractReport(ctx, root, graph)
	}}, nil
}
