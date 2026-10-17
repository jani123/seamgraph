package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"seamgraph/internal/analyzer/testresults"
	"seamgraph/internal/config"
	"seamgraph/internal/gitdiff"
	"seamgraph/internal/ir"
	"seamgraph/internal/revision"
	"seamgraph/internal/verification"
)

func evidencePath(root string) string { return filepath.Join(root, ".seamgraph", "evidence.json") }
func readStore(root string) (verification.Store, error) {
	s := verification.Store{SchemaVersion: 1, Evidence: []verification.Evidence{}}
	dirInfo, err := os.Lstat(filepath.Dir(evidencePath(root)))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if !dirInfo.IsDir() {
		return s, fmt.Errorf("evidence directory must not be a symlink")
	}
	info, err := os.Lstat(evidencePath(root))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if !info.Mode().IsRegular() {
		return s, fmt.Errorf("evidence store must be a regular file")
	}
	b, err := os.ReadFile(evidencePath(root))
	if err != nil {
		return s, err
	}
	s = verification.Store{}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&s); err != nil {
		return s, err
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return s, fmt.Errorf("trailing evidence store data")
	}
	if s.SchemaVersion != 1 {
		return s, fmt.Errorf("unsupported evidence store version")
	}
	return s, nil
}
func contractReport(ctx context.Context, root string, s ir.Snapshot) (verification.Report, error) {
	m, err := config.LoadContracts(root)
	if err != nil {
		return verification.Report{}, err
	}
	store, err := readStore(root)
	if err != nil {
		return verification.Report{}, err
	}
	rev, fingerprint, err := revision.Identity(ctx, root)
	if err != nil {
		return verification.Report{}, err
	}
	return verification.Evaluate(s, m, store, rev, fingerprint)
}

// runTests executes only fixed supported runners, never YAML command strings.
func runTests(ctx context.Context, root, runner string) (verification.Store, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return verification.Store{}, err
	}
	rev, fingerprint, err := revision.Identity(ctx, root)
	if err != nil {
		return verification.Store{}, err
	}
	if rev == "" {
		return verification.Store{}, fmt.Errorf("test evidence requires a Git commit; initialize/commit the project before collection")
	}
	artifactDir, err := safeEvidenceDir(root)
	if err != nil {
		return verification.Store{}, err
	}
	cmd, format, artifact, err := testresults.Command(ctx, root, artifactDir, runner)
	if err != nil {
		return verification.Store{}, err
	}
	if artifact != "" {
		if err := os.Remove(artifact); err != nil && !os.IsNotExist(err) {
			return verification.Store{}, err
		}
	}
	output, runErr := cmd.CombinedOutput()
	if artifact != "" {
		var readErr error
		output, readErr = os.ReadFile(artifact)
		if readErr != nil {
			return verification.Store{}, fmt.Errorf("runner did not produce an artifact: %w (runner: %v)", readErr, runErr)
		}
	}
	afterRev, afterFingerprint, err := revision.Identity(ctx, root)
	if err != nil {
		return verification.Store{}, err
	}
	if afterRev != rev || afterFingerprint != fingerprint {
		return verification.Store{}, fmt.Errorf("project inputs changed during test execution; evidence discarded")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	hash := sha256.Sum256(append([]byte(rev+fingerprint+now), output...))
	runID := hex.EncodeToString(hash[:])
	evidence, err := testresults.Import(format, output, testresults.Run{Runner: runner, Revision: rev, Fingerprint: fingerprint, RunID: runID, ObservedAt: now})
	if err != nil {
		return verification.Store{}, fmt.Errorf("cannot import runner evidence: %w (runner: %v)", err, runErr)
	}
	store, err := appendEvidence(root, evidence)
	if err != nil {
		return store, err
	}
	if runErr != nil {
		return store, fmt.Errorf("tests failed; actual evidence saved: %w", runErr)
	}
	return store, nil
}

func revisionContracts(storeRoot string) gitdiff.Verify {
	return func(ctx context.Context, root string, s ir.Snapshot, commit string) (verification.Report, error) {
		m, err := config.LoadContracts(root)
		if err != nil {
			return verification.Report{}, err
		}
		store, err := readStore(storeRoot)
		if err != nil {
			return verification.Report{}, err
		}
		_, fingerprint, err := revision.Identity(ctx, root)
		if err != nil {
			return verification.Report{}, err
		}
		return verification.Evaluate(s, m, store, commit, fingerprint)
	}
}

// importTests requires explicit run metadata, rather than relabeling an arbitrary
// old report as a fresh execution of today's tree.
func importTests(root, format, artifact, metadata string) (verification.Store, error) {
	var meta struct {
		Version        int    `json:"version"`
		Runner         string `json:"runner"`
		Revision       string `json:"revision"`
		Fingerprint    string `json:"fingerprint"`
		RunID          string `json:"runId"`
		ObservedAt     string `json:"observedAt"`
		ArtifactSHA256 string `json:"artifactSha256"`
	}
	b, err := os.ReadFile(metadata)
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
	b, err = os.ReadFile(artifact)
	if err != nil {
		return verification.Store{}, err
	}
	hash := sha256.Sum256(b)
	if meta.ArtifactSHA256 != hex.EncodeToString(hash[:]) {
		return verification.Store{}, fmt.Errorf("test artifact SHA256 does not match run metadata")
	}
	evidence, err := testresults.Import(format, b, testresults.Run{Runner: meta.Runner, Revision: meta.Revision, Fingerprint: meta.Fingerprint, RunID: meta.RunID, ObservedAt: meta.ObservedAt})
	if err != nil {
		return verification.Store{}, err
	}
	return appendEvidence(root, evidence)
}

func safeEvidenceDir(root string) (string, error) {
	dir := filepath.Join(root, ".seamgraph")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf(".seamgraph must be a real project directory")
	}
	return dir, nil
}
func appendEvidence(root string, evidence []verification.Evidence) (verification.Store, error) {
	var store verification.Store
	dir, err := safeEvidenceDir(root)
	if err != nil {
		return store, err
	}
	lock := filepath.Join(dir, "evidence.lock")
	f, err := os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return store, fmt.Errorf("evidence store is locked or unavailable (%s): %w", lock, err)
	}
	f.Close()
	defer os.Remove(lock)
	store, err = readStore(root)
	if err != nil {
		return store, err
	}
	known := map[string]string{}
	for _, e := range store.Evidence {
		b, _ := json.Marshal(e)
		known[e.ID] = string(b)
	}
	for _, e := range evidence {
		b, _ := json.Marshal(e)
		if old, ok := known[e.ID]; ok {
			if old != string(b) {
				return store, fmt.Errorf("conflicting evidence identity %s", e.ID)
			}
			continue
		}
		store.Evidence = append(store.Evidence, e)
		known[e.ID] = string(b)
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return store, err
	}
	temp, err := os.CreateTemp(dir, "evidence-*.tmp")
	if err != nil {
		return store, err
	}
	defer os.Remove(temp.Name())
	if _, err = temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		return store, err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return store, err
	}
	if err = temp.Close(); err != nil {
		return store, err
	}
	err = os.Rename(temp.Name(), evidencePath(root))
	return store, err
}
