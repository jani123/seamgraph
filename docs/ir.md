# IR v1

`internal/ir` is the sole architecture schema. `Snapshot` includes versioned
modules, seams, symbols, relations and diagnostics. Adapters implement the shared
`Analyze(context.Context, Project, config.Config) (ir.PartialSnapshot, error)`
contract and resolve language-specific facts before returning. Nothing downstream
may inspect their ASTs or import their packages. Consumers and dependencies will
be views over observed relations; violations will be policy results.

Identity rules:

- Module: `module:<declared-name>`.
- Seam: `seam:<module>.<declared-seam-name>`.
- Symbol: `symbol:<module>:<URL-PathEscape(canonical-symbol-path)>`.
- Relation: `edge:<kind>:<QueryEscape(from-ID)>-><QueryEscape(to-ID)>`.
- Violation: `violation:<full-SHA256(rule-ID + NUL + relation-ID)>`.

Use the central constructors, not string concatenation. Names cannot contain
dots or colons; encoded paths and endpoints preserve delimiters without collisions.
Validation rejects noncanonical IDs, duplicate IDs, dangling references,
ambiguous source roots, private/cross-owner contract members and invalid evidence.
Merge accepts identical definitions, deduplicates relation evidence and diagnostics,
and rejects conflicting definitions. Each module/seam definition must be complete;
adapters do not append partial membership lists to an existing identity.

Canonical symbol paths must identify declarations independent of line number.
Adapters must document how source paths and declaration names form canonical
paths. Signature edits preserve identity when the declaration path stays fixed.
Renaming a module, seam, or canonical symbol path changes its ID. Moving a symbol
changes identity if its canonical path changes; line-only moves do not. There is
no implicit rename matching, nor alias/migration support in P1.

Canonical JSON sorts entities by ID, string sets lexically, evidence and
diagnostics by their compact JSON representation; it deduplicates identical
evidence/diagnostics, emits empty arrays as `[]`, uses Go's standard JSON escaping,
and ends with a newline. Source paths use portable relative slashes. Canonical
encoding never mutates the caller. `Decode` rejects unknown fields, trailing data,
unsupported versions and invalid graphs. Schema changes require an explicit
version/migration decision; unsupported versions currently fail.

Evidence ranges use one-based line and column positions, with end positions
exclusive when available. Confidence is `exact` or `best-effort`; resolution is
`resolved`, `unresolved` or `dynamic`. Observed relations require resolved evidence.
Uncertainty belongs in diagnostics, retaining its evidence and explicit resolution,
so it cannot accidentally become a proven permitted edge. Analyzer phases must
produce these diagnostics and the policy engine implements error/warn handling.

`ArchitectureJSON` excludes diagnostic and relation evidence records for semantic
fixture comparisons; it is a projection, not a reloadable snapshot. Full `JSON`
retains all evidence. Synthetic Go/TypeScript/Python fixtures encode the same
architecture and compare to one architecture golden. These are IR contract
fixtures, not claims that any language analyzer exists. Shared policy and architecture-comparison tests now run against all three
fixtures before adapters start. Git revision isolation remains P7 work.

`Relation.bypassesSeam` optionally records an adapter-proven boundary bypass,
including a deep import of a symbol that is otherwise a public contract member.
It defaults to false and is omitted in canonical JSON when false. The common
merger rejects conflicting relation endpoints/kinds. Multiple observed access paths merge their
bypass flag with OR and retain precise `bypassEvidence` separately. A permitted
import cannot erase a deep import, nor be falsely cited as bypass evidence.
`Owners` maps modules, seams and symbols to normalized module ownership for
shared consumers without language-specific branching.

Only `imports`, `uses` and `calls` are consumer relations. `implements` is a
structural type fact and `exposes` is a declaration fact. Neither creates an
observed consumer dependency by itself. Core helpers centralize this distinction.


Code exploration adds optional v1 declaration metadata on symbols: `definition`
contains a name, canonical parent ID, source location, available documentation and
a declaration/body digest. Parent links must resolve within the same owner and
cannot form cycles. These additive fields remain optional for existing v1
producers and fixtures; strict older readers must be upgraded to consume snapshots
that contain them. Architecture projections and architectural symbol comparison
exclude declaration metadata, so body/docs/position changes do not become contract
signature changes.

Reference evidence may include a validated symbol `sourceId` and `kind`
(`reference` or `call`). The enclosing declaration and statically recognized call
syntax support focused exploration, including uses within the same module.
They do not establish runtime reachability. Module-based architecture endpoints
remain intact; the code browser projects source identities from that evidence.
Unconfigured indexing uses a synthetic `unassigned` owner, then overlays any
declared ownership using the same canonical symbol paths. Missing ownership is
shown explicitly and never creates implicit seams.
