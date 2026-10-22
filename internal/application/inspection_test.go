package application

import (
	"context"
	"errors"
	"seamgraph/internal/config"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"seamgraph/internal/verification"
	"testing"
)

func queryPorts(t *testing.T) Ports {
	t.Helper()
	doc, err := config.Parse([]byte("version: 1\nmodules:\n  app:\n    language: go\n    paths: [src]\npolicies:\n  default: deny\n"))
	if err != nil {
		t.Fatal(err)
	}
	mods, seams := doc.Config.IRModules()
	return Ports{
		Scan: func(context.Context, string) (ir.Snapshot, config.Config, error) {
			return ir.Snapshot{SchemaVersion: 1, Modules: mods, Seams: seams}, doc.Config, nil
		},
		LoadContracts:  func(string) (verification.Manifest, error) { return verification.Manifest{Version: 1}, nil },
		ReadStore:      func(string) (verification.Store, error) { return verification.Store{SchemaVersion: 1}, nil },
		Identity:       func(context.Context, string) (string, string, error) { return "commit", "fingerprint", nil },
		Initialize:     func(string) error { t.Fatal("query attempted initialization"); return nil },
		PrepareProject: func(string, []byte) (bool, error) { t.Fatal("query attempted project write"); return false, nil },
		AppendEvidence: func(string, []verification.Evidence) (verification.Store, error) {
			t.Fatal("query attempted evidence write")
			return verification.Store{}, nil
		},
		Execute: func(context.Context, string, string, string) ([]byte, string, error, error) {
			t.Fatal("query attempted test execution")
			return nil, "", nil, nil
		},
	}
}
func TestInspectionQueriesDoNotWriteOrExecuteTests(t *testing.T) {
	svc, err := New(t.TempDir(), queryPorts(t), "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	session, err := svc.InspectProject(context.Background(), svc.Root(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if _, err = session.Verify(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func TestRevisionInspectionOwnsCleanupOnSuccessAndFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failed-analysis"}[fail], func(t *testing.T) {
			ports := queryPorts(t)
			cleaned := 0
			ports.Extract = func(context.Context, string, string) (string, string, func(), error) {
				return "temporary-tree", "commit", func() { cleaned++ }, nil
			}
			if fail {
				ports.Scan = func(context.Context, string) (ir.Snapshot, config.Config, error) {
					return ir.Snapshot{}, config.Config{}, errors.New("analysis failed")
				}
			}
			svc, _ := New(t.TempDir(), ports, "test")
			session, err := svc.InspectProject(context.Background(), svc.Root(), "HEAD")
			if fail {
				if err == nil {
					t.Fatal("expected analysis error")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if cleaned != 0 {
					t.Fatal("tree closed before consumer finished")
				}
				session.Close()
				session.Close()
			}
			if cleaned != 1 {
				t.Fatalf("cleanup count %d, want 1", cleaned)
			}
		})
	}
}
func TestCanceledRevisionComparisonCleansBothTrees(t *testing.T) {
	cleaned := 0
	extract := func(context.Context, string, string) (string, string, func(), error) {
		return "temporary-tree", "commit", func() { cleaned++ }, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	analyze := func(ctx context.Context, _ string) (ir.Snapshot, policy.Policy, error) {
		return ir.Snapshot{}, policy.Policy{}, ctx.Err()
	}
	_, err := Compare(ctx, "root", "base", "head", extract, analyze)
	if !errors.Is(err, context.Canceled) || cleaned != 2 {
		t.Fatalf("err=%v cleanup=%d", err, cleaned)
	}
}
