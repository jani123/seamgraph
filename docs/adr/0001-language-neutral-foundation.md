# ADR 0001: Language-neutral foundation and explicit ownership

Status: accepted for P0/P1, 2026-10-16.

The workspace contains tooling and a specification, but no prototype. There is
no restricted parser or heuristic analyzer to preserve or migrate.

Use `internal/ir` as the sole graph schema (superseding the illustrative
`internal/model` layout). Adapters will return PartialSnapshot values to a common
merger. Policy, diff, CLI, and web must consume only IR. Schema version 1 uses
explicit module, seam, symbol, relation, evidence, and diagnostic records. Consumer
and dependency views are derived from relations, and violations will be derived
by policy; duplicating them now would create competing sources of truth.

Configuration uses yaml.v3 nodes and strict decoding, including duplicate-key
rejection. Version 1 module roots are literal project-relative directory paths.
Reject glob roots for now with an actionable error rather than invent precedence.
Nested roots belonging to different modules are rejected, including ancestor roots
within the same module. Ownership is unique or explicitly unowned. This conservative
rule is reversible when a documented glob/nesting design is agreed.

Names use ASCII letters, digits, underscore, and hyphen. Dots separate module and
seam identities and are forbidden within their names. Canonical symbol paths and
relation endpoints are URL-escaped centrally. Renames change identity; source line
moves and display-label changes do not. No implicit rename inference or migration
aliases in v1. Relation identity includes kind and endpoints, never evidence.

Canonical JSON validates references, sorts all unordered collections, removes
identical evidence, uses relative slash paths, and terminates with a newline.
Architecture projections exclude evidence and diagnostics for cross-language
fixture comparisons. Original evidence remains in normal snapshots.

P0/P1 ship `init` and `validate` to exercise configuration end to end. Analysis,
policy, verification, and serving commands fail explicitly until their phases;
no empty architecture or successful check is fabricated. Configuration includes
an explicit unresolved-reference policy, defaulting to error. Cross-language
allow declarations are intent only. Behavioral contract requirements and mappings
are deferred to P5b and will require explicit architect decisions.

No Git metadata exists in this workspace. Do not initialize Git or create an
artificial baseline solely to satisfy the requested phase commits. Record this
constraint and leave changes reviewable on disk.
