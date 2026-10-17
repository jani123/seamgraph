# SeamGraph — Agent implementation plan

Status: implementation specification v1 · 2026-10-15

## Mission

Build a **local-first Go CLI with an embedded web server** that makes large Go, TypeScript, and Python repositories understandable at the **module / seam / consumer** level, without requiring a human to read implementation details. Enforce architecture rules in CI and show architecture changes between Git revisions. The tool must be deterministic, offline-capable, and useful for AI-generated code reviews.

## Non-goals

- No LLM dependency, cloud backend, database, or hosted service.
- No general-purpose code browser, UML class diagram, or function-call graph.
- No claim that an exported symbol is automatically a properly enforced seam.
- No requirement to create an interface for every module.
- No automatic modifications to project source code.

## Required stack

- CLI, API, orchestration: Go.
- Server-rendered web UI: Go + Templ + HTMX + Tailwind CSS + Alpine.js.
- Interactive graph: Cytoscape.js or comparable small graph library, served locally (vendor/bundle assets; no CDN).
- YAML config: a real YAML parser with strict validation.
- Go analysis: `golang.org/x/tools/go/packages`, `go/types`, AST where necessary.
- TypeScript analysis: TypeScript Compiler API in a bundled Node helper. Prefer accurate type/symbol resolution over regex; report clearly if Node/TypeScript dependencies are missing. Go CLI remains the primary entrypoint.
- Python analysis: Python 3 helper using the standard `ast` module plus an explicit project import resolver; optional deeper semantic analysis must be reported as best-effort, never falsely treated as proven. Python executable availability and version must be checked.
- No persistent database in v1; snapshots can be JSON files.

## Mandatory language-neutral intermediate representation (IR)

**Hard architecture boundary:** each language analyzer must normalize its findings into the *same* versioned IR schema before they reach any other subsystem. Core, rule engine, snapshots, diff, CLI output, HTTP API and UI consume only IR. They must not switch on source language or inspect Go/TypeScript/Python ASTs. Language metadata is permitted for display and diagnostics, never for branching in core behavior.

IR entities (typed Go structs with canonical JSON serialization):

- `Module {id, displayName, sourceRoots, seams[]}` — architectural ownership, independent of source language.
- `Seam {id, moduleId, contractMembers[]}` — named public contract, with stable member IDs.
- `Symbol {id, moduleId, kind, displaySignature, visibility}` — normalized API symbol, not a raw AST node.
- `Relation {id, fromId, toId, kind, evidence[]}` — normalized `imports`, `uses`, `implements`, `exposes` and `calls` where statically supportable. Relations must not imply stronger evidence than the analyzer actually established.
- `Evidence {file, startLine, startColumn, endLine, endColumn, confidence, resolution}` — source location plus `resolved | unresolved | dynamic` status.
- `Diagnostic {code, severity, entityIds[], evidence[], message}` — analyzer uncertainty and invalid input; architecture violations are derived from IR and policy.
- `Snapshot {schemaVersion, modules[], seams[], symbols[], relations[], diagnostics[]}` — sorted, reproducible, portable serialization.

Stable IDs must derive from declared module/seam identities and canonical symbol paths, not line numbers, display labels or graph layout. Define a documented rename/move policy and canonical sorting. Keep raw language-specific payloads out of IR; if truly necessary, store optional opaque provenance solely for adapter diagnostics, never as an input to rules.

Define one adapter interface, e.g. `Analyze(ctx, project, config) (ir.PartialSnapshot, error)`. Each adapter resolves its own imports, exports, aliases, structural interface conformance and dynamic limitations before returning IR. A common merger validates referential integrity, deduplicates relations, resolves ownership and sorts output. **No Go/TS/Python conditional branches downstream of adapters.**

The rule engine must operate on IR module/seam/member identities, and must distinguish a declared dependency from a proven seam-member use. Unresolved and dynamic references must generate diagnostics and follow an explicit configurable failure policy; they must never be silently treated as allowed. Cross-language declared relationships are policy intent, not observed static usage unless an adapter supplies verifiable evidence.

### IR acceptance gates (before language adapters)

1. Define `internal/ir` schema, versioning, validation and canonical JSON encoding, with golden fixtures.
2. Create synthetic IR fixtures for identical architectures originating from Go, TypeScript and Python; assert byte-for-byte equivalent *architecture-relevant* canonical outputs after excluding source location/provenance differences.
3. Run the **same** policy tests and Git diff tests against all three fixtures; prohibit language-based special cases in these packages.
4. Add an architecture test or import-boundary check that prevents core/rules/diff/web from importing language adapter packages or language-specific parser packages.
5. Only then implement adapters incrementally, beginning with Go, then TypeScript, then Python. Each adapter must pass shared IR contract tests.

## Conceptual model

- **Module:** named architectural boundary mapped to a Go package or package subtree, or to a TypeScript/Python directory subtree. Modules must not overlap ambiguously.
- **Seam:** named public contract of a module. Supports selected exported symbols / TS barrel exports, and optional explicit Go interfaces. A seam is not necessarily a language-level `interface`.
- **Consumer:** module that references a seam, with source locations and concrete symbol evidence.
- **Dependency:** actual resolved cross-module reference/import, with evidence. Distinguish module dependency from seam usage.
- **Violation:** dependency or symbol usage disallowed by configured architecture rules, with an actionable location and explanation.
- **Boundary:** a user-declared module scope and public entrypoint/symbol set; access outside the seam is a violation, even when the underlying language permits it. Enforce only statically observable references, and report dynamic/unknown cases.
- **Snapshot:** canonical, stable serialization of the architecture graph for comparison.

Do not equate a Go package import with proof that usage goes through a declared interface. Go structural interfaces and concrete types require symbol/type analysis. TS barrels alone do not prevent deep imports; enforce those with resolved import paths.

## Example configuration (design target; adjust schema only with documented rationale)

```yaml
version: 1
modules:
  world:
    language: go
    paths: [internal/world]
    seams:
      reader:
        symbols: [WorldReader]
      commands:
        symbols: [WorldCommands]
  npc:
    language: go
    paths: [internal/npc]
  scripts:
    language: python
    paths: [src/scripts]
    seams:
      public:
        entrypoints: [src/scripts/api.py]
  ui:
    language: typescript
    paths: [src/ui]
    seams:
      public:
        entrypoints: [src/ui/index.ts]
policies:
  default: deny # explicit opt-in; also support allow for incremental adoption
  allow:
    - from: npc
      to: world.reader
    - from: scripts
      to: world.reader # illustrative cross-language declaration; no runtime edge unless analyzer finds evidence
```

Production config must support language-specific symbol and entrypoint declarations, glob/path matching with unambiguous precedence, comments preserved during interactive editing if feasible, and strict errors for unknown module/seam references. Document precisely what is and is not statically enforceable.

## CLI contract

```
seamgraph init [--root PATH]
seamgraph scan [--root PATH] [--format json|text] [--output FILE]
seamgraph check [--root PATH] [--format text|json|sarif]
seamgraph serve [--root PATH] [--addr 127.0.0.1:8765]
seamgraph snapshot [--root PATH] --output FILE
seamgraph diff --base REF --head REF [--format text|json]
```

- Exit code 0: successful check, no violations. Exit code 1: architecture violations. Exit code 2: configuration/analysis/runtime errors. Never silently treat incomplete analysis as clean.
- Git diff must compare architecture models from the two requested revisions, not raw line diffs. Use isolated Git worktrees or a safe equivalent; never alter the user's working tree. Include uncommitted-state behavior in docs.
- Bind server to loopback by default. No external telemetry. Never execute arbitrary code from YAML.

## UI requirements

1. **Architecture:** module-level graph; collapse/expand; select module to show named seams, exported contract symbols, consumers, and source evidence. No individual functions or structs in the default overview.
2. **Changes:** compare base/head revisions; highlight added/removed modules, seams, consumers, and edges, and changed public contract signatures. Hide purely internal code edits by default. A changed signature should not be represented as only an added symbol.
3. **Rules:** display allowed/denied relationships, with source evidence for violations. Clicking a relationship may propose a rule; show the exact YAML diff and require explicit confirmation before writing. Avoid destructive reformatting of unrelated YAML.
4. Search/filter by module, seam, consumer, and violation. Large graphs must remain navigable (grouping and selective expansion).
5. Server renders page chrome with Templ; HTMX for details and forms; Alpine for small local state; graph library handles canvas interactions.

## Implementation architecture

```
cmd/seamgraph/          CLI commands
internal/config/        YAML schema, validation, safe edits
internal/model/         stable language-neutral graph schema
internal/analyzer/go/   Go package and type resolution
internal/analyzer/ts/   Go wrapper for TS helper
internal/policy/        seam and dependency enforcement
internal/snapshot/      canonical graph serialization + comparison
internal/gitdiff/       safe revision checkout / analysis
internal/server/        HTTP endpoints and templ handlers
internal/ui/            templ components, bundled frontend assets
tools/ts-analyzer/      TypeScript Compiler API helper + tests
internal/analyzer/python/ Python analyzer wrapper and normalization
tools/py-analyzer/      Python AST/import resolver helper + tests
testdata/               Go/TS/Python fixture projects
```

Architecture constraints: analyzers produce factual graph data; policy engine evaluates rules; UI never reimplements policy logic. Avoid cyclic Go package imports. Analyzer errors and unresolved references must be surfaced explicitly.

## Phases — execute sequentially, small reviewable commits

### P0 — Audit prototype and establish guardrails

- Inspect the provided prototype; do not assume it is production quality.
- Add README with architecture and local build instructions, Taskfile, gofmt/go vet/go test, and test fixtures.
- Record limitations: prototype's restricted YAML parser, TS regex scanning, Go import heuristics, no seam enforcement, no diff, basic SVG.
- Set up deterministic fixtures and golden snapshots; CI runs tests offline after dependency installation.
- **Acceptance:** baseline tests and one end-to-end smoke test pass; no silent parse success on malformed input.

### P1 — Correct model, YAML, and module mapping

- Implement typed graph: Module, Seam, PublicSymbol, Consumer, Dependency, Evidence, Violation.
- Stable IDs independent of graph layout; canonical sorted JSON.
- Parse full YAML with schema validation, duplicate/overlap checks, useful diagnostics.
- Resolve source file to exactly one module or explicitly unowned; document nested-module semantics.
- **Acceptance:** round-trip configuration tests, stable snapshots, invalid config cases, and overlap cases pass.

### P2 — Reliable Go analyzer

- Resolve module/package imports using `go/packages`, including multi-package module boundaries and build tags according to selected build context.
- Discover exported public declarations, interface method signatures, references from consumer packages, and concrete types where needed for policy checks.
- Preserve file/line evidence; support Go modules and internal packages.
- Distinguish structural interface satisfaction from actual usage through a seam. Do not falsely claim full data-flow proof.
- **Acceptance:** fixture tests for allowed seam usage, forbidden concrete dependency, internal package behavior, alias imports, interface implementations, and cycles.

### P3 — Reliable TypeScript analyzer

- Build TypeScript Compiler API helper resolving tsconfig paths, barrels/re-exports, aliases, type-only imports, and cross-directory symbols.
- Map directory modules and entrypoint seams; detect deep imports bypassing configured entrypoints.
- Return same normalized graph JSON as Go adapter; helper output versioned.
- **Acceptance:** fixture tests for barrel exports, path aliases, re-exports, type-only imports, deep import violations, and ambiguous module mapping.

### P4 — Reliable Python analyzer

- Parse Python packages and modules using `ast`; map configured directory roots and declared public `__init__.py`/`api.py` entrypoints to seams.
- Resolve absolute and relative imports, `from ... import ...`, re-exports, and package aliases where statically determinable.
- Detect cross-module deep imports and references to non-public modules/symbols; include file/line evidence.
- Handle namespace packages, `src/` layouts, and configurable import roots. Dynamic imports, monkey patching, runtime attribute access, and `__getattr__` exports must be explicitly classified as unresolved/limited rather than silently accepted.
- Do not pretend Python has compiler-enforced encapsulation: seam enforcement is a static policy check.
- **Acceptance:** fixtures for relative imports, re-exports, `__all__`, private deep imports, src layouts, dynamic imports, and namespace packages; results normalize to the common graph.

### P5 — Policy engine and CLI check

- Enforce default allow/deny, explicit seam grants, disallowed module edges, and forbidden deep imports.
- Include evidence with file, line, consumer, provider, symbol, violated rule, and remediation hint.
- Support incremental adoption via allow-default and a violation baseline, with new violations failing CI.
- Text, JSON, and SARIF outputs; documented exit codes.
- **Acceptance:** violations trigger exit 1, incomplete analysis triggers exit 2, tests demonstrate no false success.

### P6 — Browser architecture explorer

- Replace inline HTML with Templ/HTMX/Tailwind/Alpine; bundle static assets locally.
- Implement module graph, seam details, consumer lists, search, evidence links, and violation highlights.
- Optimize graph rendering for at least 100 modules and 500 edges in synthetic fixtures; use grouping/filters rather than showing all at once.
- **Acceptance:** smoke test starts server, opens graph endpoint, selects module, and displays seam users; no CDN requests.

### P7 — Git architecture diff

- Analyze two refs in isolated read-only-safe worktrees or equivalent, cache by commit and config fingerprint.
- Compare stable module/seam IDs and normalized signatures; show additions/removals/changes and rule regressions.
- Provide CLI JSON/text and UI visual overlay.
- **Acceptance:** fixture repository demonstrates an added forbidden edge, renamed/changed seam, removed consumer, and an internal-only edit that does not pollute architecture diff.

### P8 — Click-to-rule editing and hardening

- Selecting edge/consumer proposes `allow` or `deny` rule.
- Preview exact YAML diff, validate it, require explicit confirmation, write atomically with backup or safe rollback.
- Ensure no source code is modified. Harden local HTTP endpoints, avoid arbitrary file reads and command injection.
- **Acceptance:** browser integration tests cover proposal, cancel, confirm, invalid config, and concurrent config change.

## P5b — Seam contract verification (mandatory v1)

**Purpose:** allow the architect to assess the verification evidence for a module's public contract without reviewing its internal implementation. A passing test suite or line coverage alone MUST NOT be presented as proof that a seam is fully tested.

### Language-neutral evidence model

Extend the versioned IR (or a separate versioned verification report keyed to the IR) with:

- `ContractRequirement {id, seamId, memberId?, description, kind}`: stable, explicitly declared behavioral expectations, including errors, boundary cases, invariants and observable side effects. API-member presence alone is not a behavioral specification.
- `TestEvidence {id, requirementIds[], memberIds[], testId, runner, sourceLocation?, revision, runId, outcome, observedAt, evidenceKind}`: normalized, traceable evidence. `evidenceKind` distinguishes test execution, coverage, static linkage and manually reviewed assertions; never conflate them.
- `VerificationResult {seamId, revision, status, memberCoverage, requirementCoverage, missingRequirements[], failingTests[], staleEvidence[], unknownEvidence[]}`.
- Allowed status values: `verified`, `partial`, `unverified`, `failed`, `unknown`, `stale`. `verified` means *all explicitly declared required contract behaviors have linked, current, passing evidence according to the configured policy*, **not** that the implementation is correct or bug-free.

All consumers (rules, CLI, HTTP, UI, diff) read this normalized model. Language-specific test runners and coverage importers belong behind adapters; no language switches in verification core. Test evidence must not be fabricated by inferring a test from its name, a call graph edge, or raw line coverage.

### Collection and verification policy

1. Support explicitly linking test cases to contract requirements using a simple YAML mapping or machine-readable test manifest. Preserve source references. Start with manually declared mappings; automatic detection may suggest candidates but must not mark them verified without validation.
2. Import actual test-run results and optional coverage artifacts through language-specific collectors: Go (`go test` / coverage), TypeScript (e.g. Vitest/Jest reports), Python (pytest/JUnit/coverage). Make supported formats explicit, and report unsupported/missing artifacts as `unknown`.
3. Require a passing test execution tied to the exact Git revision (and relevant configuration) being assessed. Changed seam signatures, relevant implementation changes, changed requirements, or changed test sources invalidate/re-evaluate evidence. If safe impact analysis is unavailable, conservatively mark evidence `stale` until rerun.
4. Report separately (a) members exercised, (b) requirements verified by passing tests, and (c) unverified/unknown behavior. An operation executed in a test is **not** automatically behaviorally verified.
5. Configurable enforcement policy: `report` initially; `fail-on-failed`, `fail-on-missing`, and `require-verified` for selected seams in CI. Incomplete analysis must never silently pass a strict policy.
6. Deterministic JSON output with stable IDs, evidence paths, exact failing test references, and machine-readable diagnostic codes.

### CLI and UI

- `seamgraph inspect seam:world.reader --tests --json`: show members, declared requirements, linked tests, execution outcomes, missing coverage, and freshness.
- `seamgraph check --contracts --json`: enforce configured contract verification policy and return nonzero on violations or strict-policy unknowns.
- `seamgraph diff --base HEAD --json`: show newly exposed or modified seam members, changed requirements, and verification regressions.
- Browser seam details: clear `verified / partial / unverified / failed / unknown / stale` status; member-by-member and requirement-by-requirement evidence, clickable test source locations, and a copyable stable seam ID for agent follow-up.
- UI must label percentages precisely: **member execution coverage** and **declared contract requirement verification** are different metrics. Never show a single unqualified "seam coverage" percentage.

### Acceptance tests

- A 95% line-covered module with an unverified public operation is **not** verified.
- A test that executes an operation without checking required behavior does **not** verify its contract.
- A failing required test yields `failed`; absent artifacts yield `unknown` or `unverified`, never `verified`.
- Changing implementation after a passing run yields `stale` until valid evidence is collected again.
- Equivalent Go, TypeScript and Python test-evidence fixtures yield identical normalized verification outcomes and policy decisions.
- `check --contracts` works offline, without a running web server or MCP.

### Agent workflow

After code changes, run relevant tests, collect evidence, then run `seamgraph check --contracts --json` and report the seam statuses. Do not weaken rules, alter test-to-requirement mappings, or claim verification without explicit user approval and reproducible evidence. If a requirement is missing or ambiguous, ask for a contract decision rather than inventing one.

## Required tests and review process

- Unit tests for model/config/policy/diff; integration fixtures for Go, TypeScript, and Python; CLI end-to-end tests.
- Each phase ends with `go test ./...`, Go vet, TS/Python test suites where applicable, and a short manual UI smoke test for UI phases.
- Commit at phase boundaries, preferably with smaller logical commits within phases.
- After each phase write `docs/agent-progress.md` with: completed behavior, files touched, tests run and results, architectural changes, known limitations, next phase.
- If an architectural choice is ambiguous, document an ADR and prefer the simplest reversible implementation. Do not silently invent semantics for seam enforcement.
- Do not mark a phase complete with failing tests or unresolved analysis errors. Do not implement later phases to hide earlier failures.

## Definition of done (v1)

Contract verification is required for v1: normalized test evidence, revision freshness, seam/member/requirement reporting, CLI `inspect --tests` and `check --contracts`, and the shared three-language verification fixture suite must pass.


Given a Go, TS, or Python fixture project, the user can configure module boundaries and seams in YAML; scan to see modules, public contracts and real consumers; detect forbidden access through imports/symbols with source evidence; inspect the graph locally; compare two Git revisions at the architecture level; and run `check` in CI. All results must be deterministic and reproducible without AI or network access at runtime.

## Boundary enforcement semantics (mandatory)

- A module is defined by explicit paths; it may expose one or more named seams. Every cross-module source reference must resolve to an allowed seam or be flagged under strict policy.
- The policy engine distinguishes **declared allowed dependency**, **observed reference**, and **observed bypass**. A permitted module-to-module edge does not automatically permit access to every exported symbol.
- Go: exported identifiers are language-public, but SeamGraph can further restrict cross-module usage by symbol and package path. Interfaces are optional; an exported constructor/function/type may be part of a seam. `internal/` provides only Go-native directory restrictions.
- TypeScript: enforce entrypoint imports and symbol re-exports; prevent deep imports even when TypeScript resolves them. Handle type-only references.
- Python: enforce public import paths and configured exports with static AST/import resolution; do not infer runtime guarantees.
- Unknown/unresolved references must appear in diagnostics. Strict mode fails on unresolved cross-boundary analysis; relaxed mode emits explicit warnings.
- Use fixture-based negative tests proving each language rejects a bypass and positive tests proving permitted access works.

## Agent kickoff instruction

> Read this plan and inspect the existing prototype. Start with P0 and P1 only. Produce a concise architecture ADR before making major changes. Implement in small commits, run the specified tests, and write `docs/agent-progress.md`. Stop after P1 and report what is complete, what is uncertain, and what requires a design decision. Do not start P2 until the P1 model and YAML schema are tested.

## P9 — Copyable architecture IDs and agent inspection (mandatory v1)

**Goal:** Every architecture element visible in the browser can be referenced unambiguously in an agent prompt, without the agent needing the browser.

- Assign canonical, stable, human-readable IDs to modules, seams, dependency edges, and violations. Suggested forms: `module:world`, `seam:world.reader`, `edge:npc->world.reader`, and `violation:<stable-fingerprint>`. Define escaping/normalization and collision detection centrally in the IR package. IDs must not be based on display coordinates or ephemeral source line numbers.
- Preserve IDs across scans and commits when the logical element remains unchanged. A rename is an explicit identity change unless an alias/migration is declared; never silently conflate distinct elements.
- Clicking a node, seam, or edge shows its ID and a one-click **Copy ID** action. Also provide **Copy agent prompt**, e.g. `Inspect seam:world.reader: explain its contract, consumers, source references, and violations; do not modify code.`
- Add `seamgraph inspect <id> [--json] [--root PATH] [--revision REF]`. The CLI resolves the same ID against the same canonical IR, and returns contract details, consumers, source locations, and rule diagnostics. Unknown/ambiguous IDs produce actionable nonzero errors and candidate IDs where safe.
- JSON output is machine-readable with a versioned schema and deterministic ordering. Text output is concise for direct use by an agent. No LLM or network access is required.
- In Git diff view, both before/after identities are available. Removed elements can be inspected with `--revision` (or an equivalent explicit baseline selector).
- The browser must not expose arbitrary source files through IDs; source locations are constrained to the scanned project root.
- **Acceptance:** for Go, TS, and Python fixtures, copy an ID from the UI and resolve it via CLI to the same IR element; verify stable IDs across an internal-only change, explicit behavior on rename, and errors on unknown IDs. Browser tests verify copy actions for nodes and edges.

**Implementation order:** Define ID semantics and IR support in P1, add CLI inspection with the CLI phase, add copy actions with the UI phase, and finish Git revision inspection with the diff phase. P9 is a cross-cutting acceptance milestone, not a reason to defer the underlying ID design.
