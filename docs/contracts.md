# Explicit contract evidence

Contract verification is separate from architecture enforcement. A seam is
`verified` only when every explicitly declared requirement has current passing
execution evidence linked through a reviewed mapping. This does not prove the
implementation is correct, cover undeclared behaviors, or establish absence of
bugs. No mapping is inferred from a test name, call edge or line coverage.

Create `seamgraph-contracts.yaml`. For example, for a project whose intended
behavior is explicitly specified as follows (adapt to your actual contract):

```yaml
version: 1
requirements:
  - id: behavior:world.read-missing
    seamId: seam:world.reader
    description: Reading an unknown identifier returns the documented missing error
    kind: error
links:
  - testId: example.com/project/world/TestReadMissing
    requirementIds: [behavior:world.read-missing]
    memberIds: []
    reviewed: true
policies:
  seam:world.reader: require-verified
```

Requirements can optionally specify `memberId`. Test mappings can optionally
include `sourceLocation` with file, startLine/startColumn/endLine/endColumn,
confidence (`exact` or `best-effort`) and resolution (`resolved`). Sources must be
project-relative. Review means the linked test's assertions check those declared
behaviors; executing a member without checking its behavior is insufficient.
`memberIds` declare linkage and never constitute measured execution coverage.

Supported policies per seam are `report` (default), `fail-on-failed`,
`fail-on-missing`, and `require-verified`. Every enforcing mode fails on unknown/stale execution evidence; `report` leaves
those states visible without enforcement. Missing declared mappings are enforced
by fail-on-missing/require-verified. JSON includes explicit status codes, exact test IDs, current
and historical evidence, missing requirements and separate metrics. With no declared
requirements a seam stays `unverified`; absent artifacts for a reviewed mapping
produce `unknown`. Failed current tests yield `failed`; partially verified
requirements yield `partial`; outdated evidence yields `stale`.

## Collect a real run

Commit the project and manifest, install its ordinary test dependencies, then:

```sh
seamgraph test --root /path/to/project --runner go
seamgraph test --root /path/to/project --runner vitest
seamgraph test --root /path/to/project --runner pytest
seamgraph check --root /path/to/project --contracts --json
seamgraph inspect seam:world.reader --root /path/to/project --tests --json
```

Choose the runner appropriate to the project; mixed projects can collect multiple
runs. Commands are fixed (`go test -count=1 -json ./...`, offline npm exec Vitest,
or Python pytest with JUnit), never YAML shell strings. Go runs bypass the result
cache. Vitest/pytest must already be installed in the project's environment. Runs
write `.seamgraph/evidence.json` under an exclusive writer lock and atomically; failing runs retain their actual
outcomes. Input changes during a run discard the evidence. A project without a Git
commit cannot collect revision-bound evidence.

Freshness checks the exact commit and a conservative hash of project files,
including architecture/contract config, implementation and tests. `.git`,
`.seamgraph`, node_modules, build/environment caches and Python caches are excluded. Selected build/test environment
values also participate, so changing build tags or target context invalidates old
evidence.
Arbitrary generated files elsewhere can conservatively invalidate a run. Put test
artifacts in `.seamgraph`. Symlinks are rejected by freshness hashing. Historical
runs remain inspectable; the latest matching current execution is authoritative.

## Import external results

Import formats: `go-test-json`, `vitest-json`, `jest-json`, `junit`. Exact test IDs
are Go `package/TestName`, Vitest/Jest fullName (duplicates rejected), and JUnit
`classname/name`. Unsupported/empty/malformed artifacts fail explicitly.

```sh
seamgraph fingerprint --root /path/to/project --json
seamgraph import-tests --root /path/to/project \
  --format junit --artifact .seamgraph/pytest.xml --metadata .seamgraph/run.json
```

The producing runner must capture revision/fingerprint at execution time, verify
inputs do not change during the run, and emit metadata:

```json
{
  "version": 1,
  "runner": "pytest",
  "revision": "<exact full Git commit>",
  "fingerprint": "<fingerprint captured for that run>",
  "runId": "<unique producer run identity>",
  "observedAt": "2026-10-16T12:00:00Z",
  "artifactSha256": "<SHA256 of the exact artifact bytes>"
}
```

The importer checks the artifact digest and never automatically labels an old
report as a run against today's tree. Metadata is producer-supplied traceability,
not cryptographic attestation that a test was honestly executed.

## Coverage and Git comparison

The normalized evidence model keeps `coverage`, `static-linkage`,
`reviewed-assertion`, and `test-execution` distinct. Only test execution with a
reviewed requirement mapping verifies behavior. Member execution is unknown
without current member-level coverage evidence; the UI labels that explicitly.
Built-in coverage-file importers are not yet implemented; current CLI test
collectors import execution outcomes only. This optional coverage support is a
remaining extension, not a fabricated percentage.

Git comparison reports requirement edits and verification status/metric changes
using each revision's manifest and locally retained evidence. Run timestamps alone
do not change the architecture diff. Removed elements can be inspected using
`inspect <id> --revision REF --tests --json`.
