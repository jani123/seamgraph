package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"seamgraph/internal/ir"
	"seamgraph/internal/verification"
	"time"
)

func (svc *Service) contractReport(ctx context.Context, root string, s ir.Snapshot) (verification.Report, error) {
	m, err := svc.ports.LoadContracts(root)
	if err != nil {
		return verification.Report{}, err
	}
	store, err := svc.ports.ReadStore(root)
	if err != nil {
		return verification.Report{}, err
	}
	rev, fingerprint, err := svc.ports.Identity(ctx, root)
	if err != nil {
		return verification.Report{}, err
	}
	return verification.Evaluate(s, m, store, rev, fingerprint)
}

// runTests executes only fixed supported runners, never YAML command strings.
func (svc *Service) RunTests(ctx context.Context, root, runner string) (verification.Store, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return verification.Store{}, err
	}
	rev, fingerprint, err := svc.ports.Identity(ctx, root)
	if err != nil {
		return verification.Store{}, err
	}
	if rev == "" {
		return verification.Store{}, fmt.Errorf("test evidence requires a Git commit; initialize/commit the project before collection")
	}
	artifactDir, err := svc.ports.EvidenceDir(root)
	if err != nil {
		return verification.Store{}, err
	}
	output, format, runErr, err := svc.ports.Execute(ctx, root, artifactDir, runner)
	if err != nil {
		return verification.Store{}, err
	}
	afterRev, afterFingerprint, err := svc.ports.Identity(ctx, root)
	if err != nil {
		return verification.Store{}, err
	}
	if afterRev != rev || afterFingerprint != fingerprint {
		return verification.Store{}, fmt.Errorf("project inputs changed during test execution; evidence discarded")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	hash := sha256.Sum256(append([]byte(rev+fingerprint+now), output...))
	runID := hex.EncodeToString(hash[:])
	evidence, err := svc.ports.Import(format, output, EvidenceRun{Runner: runner, Revision: rev, Fingerprint: fingerprint, RunID: runID, ObservedAt: now})
	if err != nil {
		return verification.Store{}, fmt.Errorf("cannot import runner evidence: %w (runner: %v)", err, runErr)
	}
	store, err := svc.ports.AppendEvidence(root, evidence)
	if err != nil {
		return store, err
	}
	if runErr != nil {
		return store, fmt.Errorf("tests failed; actual evidence saved: %w", runErr)
	}
	return store, nil
}

func (svc *Service) revisionContracts(storeRoot string) Verify {
	return func(ctx context.Context, root string, s ir.Snapshot, commit string) (verification.Report, error) {
		m, err := svc.ports.LoadContracts(root)
		if err != nil {
			return verification.Report{}, err
		}
		store, err := svc.ports.ReadStore(storeRoot)
		if err != nil {
			return verification.Report{}, err
		}
		_, fingerprint, err := svc.ports.Identity(ctx, root)
		if err != nil {
			return verification.Report{}, err
		}
		return verification.Evaluate(s, m, store, commit, fingerprint)
	}
}

// importTests requires explicit run metadata, rather than relabeling an arbitrary
// old report as a fresh execution of today's tree.
func (svc *Service) ImportTests(root, format, artifact, metadata string) (verification.Store, error) {
	var meta struct {
		Version        int    `json:"version"`
		Runner         string `json:"runner"`
		Revision       string `json:"revision"`
		Fingerprint    string `json:"fingerprint"`
		RunID          string `json:"runId"`
		ObservedAt     string `json:"observedAt"`
		ArtifactSHA256 string `json:"artifactSha256"`
	}
	b, err := svc.ports.ReadFile(metadata)
	if err != nil {
		return verification.Store{}, err
	}
	if err = json.Unmarshal(b, &meta); err != nil {
		return verification.Store{}, err
	}
	if meta.Version != 1 || meta.Runner == "" || meta.Revision == "" || meta.Fingerprint == "" || meta.RunID == "" || meta.ObservedAt == "" {
		return verification.Store{}, fmt.Errorf("complete version-1 test run metadata is required")
	}
	if _, err = time.Parse(time.RFC3339Nano, meta.ObservedAt); err != nil {
		return verification.Store{}, fmt.Errorf("invalid observedAt: %w", err)
	}
	b, err = svc.ports.ReadFile(artifact)
	if err != nil {
		return verification.Store{}, err
	}
	hash := sha256.Sum256(b)
	if meta.ArtifactSHA256 != hex.EncodeToString(hash[:]) {
		return verification.Store{}, fmt.Errorf("test artifact SHA256 does not match run metadata")
	}
	evidence, err := svc.ports.Import(format, b, EvidenceRun{Runner: meta.Runner, Revision: meta.Revision, Fingerprint: meta.Fingerprint, RunID: meta.RunID, ObservedAt: meta.ObservedAt})
	if err != nil {
		return verification.Store{}, err
	}
	return svc.ports.AppendEvidence(root, evidence)
}
