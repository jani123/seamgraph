# SeamGraph checks its own architecture

The root [seamgraph.yaml](../seamgraph.yaml) declares **20 modules and 31 named
seams** for the production Go application and release utility. Roots are literal
and disjoint; policy is default-deny and unresolved references are errors. Every
allow names a seam. There are no module-wide grants or violation baselines.

```sh
task architecture:check
task check
```

The checks run in CI. They enforce the implementation boundaries, not just the
appearance of the graph. The [original implementation specification](../SEAMGRAPH_PLAN.md)
still defines product behavior; the refactoring rationale follows below.

## Current ownership

| Module | Root | Responsibility |
| --- | --- | --- |
| cli | `cmd/seamgraph` | Flags, command dispatch, formatting, exit codes and HTTP process lifecycle |
| bootstrap | `internal/bootstrap` | Concrete construction and adapter registration |
| application | `internal/application` | Inspection, revision, evidence and rule workflows; capability ports and explorer response data |
| analysis | `internal/analysis` | Injected adapter dispatch, common protocol/builders, deterministic merge |
| go-adapter | `internal/analyzer/golang` | Go resolution and normalization |
| typescript-adapter | `internal/analyzer/typescript` | Bundled compiler helper and TypeScript normalization |
| python-adapter | `internal/analyzer/python` | Bundled AST helper and Python normalization |
| project | `internal/config` | Pure document parsing/validation and project definitions |
| project-files | `internal/projectfiles` | Project/source/evidence file access, discovery and atomic edits |
| git-workspaces | `internal/gitworkspace` | Controlled Git, exact trees, remote workspaces and input identity |
| test-runners | `internal/analyzer/testresults` | Fixed runners and normalized evidence import |
| model | `internal/ir` | Versioned IR, IDs, canonicalization and validation |
| policy | `internal/policy` | Pure architecture decisions |
| comparison | `internal/snapshot` | Pure architecture/policy/verification comparison |
| verification | `internal/verification` | Pure assessment of declared contract evidence |
| web | `internal/server` | HTTP routing, request security and rendering |
| ui | `internal/ui` | Templates, local assets and graph presentation |
| reporting | `internal/report` | Text/JSON/SARIF formatting and baseline application |
| runtimes | `internal/runtimecheck` | Runtime selection and availability checks |
| release-tools | `scripts/packagezip` | Independent release packaging |

Concrete adapters and file/Git/runner implementations are connected only by
bootstrap. Analysis receives its registry rather than importing implementations.
Application calls consumer-owned ports and imports no concrete implementation.
Pure engines import neither adapters nor infrastructure nor presentation.

CLI invokes the application's inspection, comparison, project and evidence
seams. Web receives an explorer capability with read, comparison and rule-edit
operations; it cannot collect test evidence or initialize projects. CLI cannot
confirm rule edits. UI receives application view data and selected domain data
seams, with no evaluation or application execution grants.

Domain data references remain intentional: a renderer may read IR fields and
verification results. Application view types contain selected domain values,
not Git handles or infrastructure result types. No duplicate copy of the whole
IR schema was introduced to make dependency counts smaller.

## Seam behavior

| Seam | Behavioral contract |
| --- | --- |
| `application.inspection` | Current/revision inspection performs no project writes or test execution. Revision sessions own temporary trees; callers close them, and close is idempotent. Failure cleans up before returning. |
| `application.explorer` | Produces normalized explorer data and evidence-backed source text. Unknown, invalid, oversized and escaping source paths fail explicitly. It never collects new test evidence. |
| `application.comparison` | Materializes exact committed trees, analyzes both, compares normalized results, and cleans both trees on success, failure or cancellation. Session caching includes commit, document digest and execution context; incomplete analysis is not cached. |
| `application.project` | Explicit CLI project initialization/opening, validation, language selection and identity. Initialization never overwrites an existing configuration. Remote preparation preserves the persistent workspace and reports its path on document errors. |
| `application.evidence` | Explicitly runs only supported fixed runners or imports artifacts with checked metadata/hash. It rejects changing revision/fingerprint inputs and saves actual failed-test evidence before reporting runner failure. |
| `application.rules` | Propose previews without writing; cancel invalidates; confirm consumes exactly one proposal and checks original bytes before atomic replacement. Conflicts require a new proposal. Proposals are bounded and serialized per service session. |
| `application.views` | Selected response values only. The service type and its operational methods are absent; view access does not imply command/query execution access. |
| `application.ports` / `.construction` | Bootstrap supplies capabilities once. Infrastructure owns file/process effects and cleanup; workflows own sequencing. Consumers do not construct implementations themselves. |
| `analysis.scan` / `.adapter-protocol` | Dispatches registered project languages in deterministic order; missing adapters fail explicitly. Each adapter returns normalized partial IR; merge validates the complete graph. No adapter imports another adapter. |
| Engine evaluation/comparison seams | Consume normalized values and return decisions/changes/coverage without file, process or network access or language-specific branching. |

Go seam selection includes exported fields/methods when selecting a type. The
service type is therefore confined to construction; read-only presentation gets
selected value types, and operational seams select receiver-qualified methods.
These behavioral contracts are covered by tests; no root contract-evidence
manifest is fabricated to claim automatic certification of these descriptions.

## Enforcement and presentation

Synthetic policy cases guard intended grants. Additional tests copy production
source with the actual root policy, verify a clean scan and an acyclic module
dependency graph, then inject real forbidden references: application → Git,
UI → query/evaluation, adapter → adapter, and CLI → project-file editing. These
must produce violations rather than being allowed by a widened seam. Static
import tests also prohibit I/O and infrastructure in pure/application modules.

The explorer uses deterministic dependency layers; strongly connected components
remain visible together rather than dropping their edges. Within those columns,
up to six forward/backward barycenter passes order modules near their consumers
and providers.
Long dependencies receive temporary layout points in intervening columns. A
crossing score counts strict segment inversions, excluding shared endpoints, and
the best complete arrangement is retained. Up to three adjacent-swap passes
(capped at 2,000 attempts) refine it using only affected incident edges. Cycle
members remain one contiguous block. Weighted neighbor alignment then adjusts
vertical positions while enforcing label spacing and preserving the order.

Module IDs break ordering ties, so shuffled input and repeated filtering produce
the same layout for the same visible graph. Hover only changes styles. Temporary
points never become architecture nodes or references. The crossing score applies
to the layered layout segments. Original relationships are rendered as orthogonal
polylines with 90-degree turns. Routing distributes ports along node sides and
chooses clear horizontal corridors around intermediate modules, with extra space
for arrowheads and labels. Cycles and expanded seams can use opposite sides.
Dragging or selecting a module updates the routes without reordering modules.
Overlapping dragged nodes may require a fallback path; the original edges remain
available. Routing adds no architecture nodes or dependencies.
Tests independently compare two-layer straight-line intersections, long edges,
cycles, shared providers and a deterministic 100-module/500-edge graph. The view selector
separates callable references, data references, imports and declared construction
wiring. Callable references include function values and do not prove execution.
A construction-only module whose seams contain function factories is wiring;
exclusive members of `.construction` seams are construction references. Types
shared with ordinary seams retain their ordinary reference categories.

Filtered views hide modules without matching connections. **All references**
retains the complete module graph, including isolated modules. Module or seam selection
focuses neighbors; **Clear focus** restores the view. Expanded seams show their
actual consumers with the original reference categories. Source evidence remains
available through relationship details regardless of the current projection.

Before refactoring, the self-scan exposed 16 modules, 23 seams and 43 directed
module pairs. After splitting hidden adapter/implementation boundaries, the graph
has more explicit pairs; that increase is expected. Success is stronger ownership
and enforced capability seams, not a lower aggregate edge count.

Production Go source and generated Templ Go are checked. Embedded CommonJS,
Python source and browser JavaScript are not independently scanned as source
modules by this self-policy. Their helper, runner, browser and graph projection
suites remain required. Build embedding and subprocess use are not invented
cross-language static references.

## Refactoring plan — 2026-10-18

Status: implemented on 2026-10-18. This section retains the diagnosis and
implementation rationale. The current boundaries and validation are documented
above; the responsibilities below remain design constraints for future work.

### Diagnosis

A fresh self-scan reports 16 modules, 23 seams and 43 distinct directed module
pairs for `imports`, `uses` and `calls`. CLI reaches 13 other modules, web reaches
6, and UI reaches 5. These are observed static references, not a runtime call
graph. The picture combines orchestration, type references and implementation
dependencies; a shared graph type naturally has many consumers.

The current allowlist mostly documents the implementation we already have. It
protects selected APIs, but that alone does not establish good responsibilities.
The most useful changes are:

- `cmd/seamgraph/main.go` performs scanning, evaluation, revision comparison and
  explorer assembly. `contracts.go` also owns evidence persistence, test execution
  and revision/fingerprint validation. A command adapter owns application logic.
- `internal/gitdiff` combines exact Git tree extraction, analysis orchestration,
  policy evaluation/comparison, verification comparison and caching. Repository
  acquisition consequently depends on a comparison module just to extract files.
- `internal/server` directly reads source files and prepares/commits configuration
  edits. HTTP handling owns project operations and their lifetime.
- UI renders `gitdiff.Result`; an infrastructure/workflow type has become a public
  presentation contract. Configuration produces policy and verification types,
  coupling document parsing to several consumers.
- The single `analysis` root hides dependencies between adapter dispatch, shared
  builders, Go/TypeScript/Python adapters and test runners. The checker cannot
  enforce boundaries within that root.

### Target ownership and seams

Keep this a single Go binary with local helpers. No services, plugin framework,
generic event bus or interface-per-function conversion is needed. Each module
owns a capability and exposes selected symbols plus documented behavior.

| Module | Owns | Public seams |
| --- | --- | --- |
| application | Project use cases and their sequencing; current/revision inspection, comparison, evidence collection, rule proposals | `inspection`, `explorer`, `comparison`, `project`, `evidence`, `rules`, `views`, `ports`, `construction` |
| analysis | Language-neutral scan coordination, adapter protocol, ownership/build helpers and deterministic merge | `scan`, `adapter-protocol` |
| go-adapter / typescript-adapter / python-adapter | Language-specific resolution and normalization | Each exposes `construction`; execution is through the shared adapter protocol |
| model | Canonical IR, IDs, validation, serialization and merge | `graph` |
| policy | Pure policy decisions over normalized input | `evaluation`, selected result types |
| comparison | Pure architecture/policy/verification comparison over supplied values | `comparison`, selected change types |
| verification | Pure contract/evidence assessment | `evaluation`, selected manifest/evidence/result types |
| project-files | YAML, discovery, source access, rule edit preparation/commit and evidence file persistence | Implement application project/evidence ports; expose `construction` |
| git-workspaces | Remote acquisition, exact revision materialization, revision identity and controlled Git execution | Implement application workspace/revision ports; expose `construction` |
| test-runners | Fixed supported runners and evidence import; no policy evaluation | Implement application evidence collection port; expose `construction` |
| web | HTTP routing, request validation, Host/Origin/CSRF protection and rendering | `http` |
| ui | Templ components and browser assets consuming application view data | `components`, `assets` |
| cli | Flag parsing, output selection, exit-code mapping and command dispatch | Command entrypoint |
| bootstrap | Concrete construction, adapter registration and resource shutdown | `run` |

Reporting and runtime checks remain small supporting modules. Release packaging
remains independent. Consolidate existing Git packages under `git-workspaces`
because they share a filesystem/process capability; do not publish Git process
construction as a general application API.

`application` is a coordinator, not a replacement dumping ground. Separate its
inspection, revision, evidence and rule workflows into cohesive operations with
explicit inputs and ports. It must not contain parsers, Git commands, HTTP handlers,
templates, direct filesystem persistence or a switch on source language.

Application `views` are deliberately selected read models for explorer and CLI
responses. Start by removing `gitdiff.Result` from presentation signatures.
Do not embed infrastructure objects or entire engine results in a view and then
claim the dependency disappeared. Preserve the existing versioned JSON shape
through explicit mapping where compatibility requires it. Internal engine-to-engine
IR and result references are legitimate; do not copy all domain models merely
to reduce graph edges.

Consumer-owned interfaces belong at the application or analysis boundary that
needs them. For example, analysis owns the adapter protocol; the Git implementation
implements application revision/workspace ports. Pure engines need no storage
interfaces. Constructors and concrete implementations are visible to bootstrap,
not ordinary workflow consumers.

### Dependency rules

The intended operational direction is:

```text
CLI ────────────────┐
                   ├──> application queries/commands ──> pure engines ──> model
HTTP ───────────────┘              │
  └──> UI ──> application.views   └──> consumer-owned ports

Go / TypeScript / Python adapters ──> analysis.adapter-protocol + model
project-files / git-workspaces / test-runners ──> application.ports
bootstrap ──> construction seams; inject implementations into consumers
```

The port arrows describe static ownership. At runtime application calls an
injected implementation; this is not a reason to allow application imports of
that implementation. Actual port signatures may also require explicit access to
domain data seams; record those legitimate references rather than hiding them.

Enforce these invariants:

1. CLI and web invoke application operations, never concrete language adapters,
   Git helpers, evidence stores or YAML edit implementations.
2. UI receives view values; it cannot execute application queries, commands or
   engine evaluation. Web retains HTTP security; application owns proposal
   lifecycle, stale-file checks and confirmed commit semantics.
3. Policy, comparison and verification perform no filesystem/process/network I/O
   and do not import application, configuration, adapters, Git or presentation.
4. Each language adapter depends on the common protocol, not another adapter or
   its parser/compiler. Test runners are separate from source analysis.
5. Project-file loading parses and validates documents. Application converts
   document values into engine inputs; parsing does not call engine evaluation.
6. Git materialization knows nothing about policy, verification or comparison.
   Application owns revision comparison orchestration and cache lifecycle.
7. Bootstrap may see many constructors; business workflows do not live there.
   Distinguish its wiring edges in the graph without excluding them from checks.
8. Every cross-module reference is covered by a named seam. Default-deny and
   unresolved-error remain; no module-wide grants or new violation baselines.

### Implementation sequence

These are ordered changes toward the complete target, not stopping points that
require separate permission or partial-delivery milestones.

1. **Record behavior and architecture before moving code.** Retain canonical
   mixed-language snapshots, CLI JSON/exit behavior, revision comparison outputs,
   evidence freshness rules, rule confirmation/conflict handling and HTTP security
   checks. Save the observed module pairs and distinguish data references from
   operations using resolved symbols where supportable; imports alone do not
   prove a call. Write the target prohibited connections alongside existing rules.
2. **Separate Git access from revision comparison.** Move tree extraction,
   acquisition, identity and process handling into the Git implementation boundary.
   Move comparison orchestration and its session cache to application. Repository
   access must no longer import comparison. Keep exact-tree and cache-invalidation
   tests, including configuration and execution-context changes.
3. **Extract application operations from CLI.** Move inspect/check/compare and
   evidence workflows behind queries/commands. Move evidence I/O and runner
   execution behind narrow ports. CLI retains flags, formatting and exit codes.
   Bootstrap wires concrete services once; use cases do not construct adapters.
4. **Separate presentation and project mutations.** Replace web callbacks and
   infrastructure result signatures with application query/command/view seams.
   Move source reads, proposal state and edit commits behind application ports.
   Preserve one-use confirmations, cancel behavior, bounded proposals, source-path
   containment, stale-file detection and HTTP request protections. Browser graph
   data must still retain entity IDs, seam members and source evidence.
5. **Split analysis into enforceable modules.** Move shared protocol/builders out
   of the parent adapter root into a disjoint analysis directory. Give Go,
   TypeScript, Python and test runners separate literal roots. Register adapters
   at bootstrap; analysis dispatches through the protocol. Run the same shared
   adapter tests for all three languages and retain uncertainty diagnostics.
6. **Tighten project-file and engine boundaries.** Separate document parsing from
   engine input conversion. Remove obsolete exports, callbacks and grants as
   callers migrate. Document each seam's inputs, outputs, errors, effects,
   cancellation and ownership/cleanup obligations. Add behavioral tests for
   these obligations, rather than treating an export list as a complete contract.
7. **Make the result readable and enforce it.** Update `seamgraph.yaml` and
   boundary tests with each code move, then audit the final graph against the
   target. Add cycle detection at module level, including cycles that Go permits
   between packages belonging to different architectural modules. Add layered
   graph layout, seam-level focus and separate filters for model references,
   operational uses and bootstrap wiring. Filters must preserve evidence and
   must not relabel uncertain references as proven calls.

All paths remain literal and non-overlapping. In particular, do not declare both
`internal/analyzer` and its children as modules: move the shared parent files or
use separate sibling directories first. Moves will change the self-snapshot;
review those expected changes without changing the IR schema or user project
identities unnecessarily.

### Completion criteria

- No production workflows or evidence storage remain in CLI; web performs no
  project file mutation or Git operation directly.
- UI has no dependency on Git, project-file implementations, adapters or engine
  execution seams. Pure engines have no infrastructure/presentation imports.
- Git access has no policy, comparison or verification dependencies. No adapter
  can import another adapter. Module-level operational dependencies are acyclic.
- Root self-policy and real forbidden-import probes reject those connections;
  the probes cover production code paths, beyond synthetic policy fixtures.
- Tests cover query immutability, cancellation/cleanup of revision workspaces,
  concurrent/stale rule edits, evidence identity/freshness, all three adapter
  contracts, and unchanged CLI/API/browser behavior. Run the full required checks
  and Go race tests after the integrated refactoring.
- The explorer shows a clear entrypoint → use case → engine flow, with data and
  construction references available separately. Edge count alone is not a pass
  criterion: moving every reference into a shared bag of DTOs would not improve
  ownership or behavioral contracts.
