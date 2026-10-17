# ADR 0004: Explicit behavior mappings and conservative freshness

Status: implemented, 2026-10-16.

Contract requirements are architect-authored, never inferred from public API
presence or test names. A separate strict YAML manifest holds stable requirements,
reviewed test mappings and per-seam enforcement policy. Production requirements
were not invented in this workspace. Synthetic acceptance-test requirements are
clearly local fixture behavior, not declarations about arbitrary user code.

Collectors adapt Go JSON, Vitest/Jest JSON and pytest/JUnit results into versioned
evidence. Test execution, measured coverage, static linkage and assertion review
remain distinct. A passing mapped test verifies only its explicitly reviewed
requirements. A member link alone does not become measured execution coverage;
without a current coverage artifact the UI displays member execution as unknown.
Raw coverage-file importers are optional follow-up work, not silently approximated.

The built-in test command uses fixed runner invocations. Go result caching is
disabled. Input hash and exact Git revision are captured before/after execution;
changed inputs discard evidence. A producer-supplied report import additionally
requires run metadata and the artifact's exact SHA256. Metadata supplies traceability,
not a claim of cryptographic execution attestation.

Freshness conservatively hashes project inputs and selected runtime/build/test
environment. It excludes Git metadata (including worktree .git files), generated
.seamgraph evidence, dependency/build/environment caches and Python caches.
Actual source, requirements, config and tests participate. This avoids inventing
impact analysis. Store updates use a process-independent exclusive lock, temp
file, fsync and atomic rename; duplicate identical imports are idempotent.

Current evidence is selected by exact revision/input fingerprint, then latest
observed execution per test/kind. Historical reports remain inspectable and do
not invalidate a newer passing run. Failing required evidence wins over passing
claims. Verification percentages/ratios are always qualified by what they measure.
Strict unknown/stale evidence fails enforcement with an explicit nonzero result.
