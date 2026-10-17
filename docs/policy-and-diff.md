# Shared policy and snapshot comparison cores

These cores run offline against validated IR only. The CLI and browser now use
them for checks and committed Git comparison.

`config.Config.Policy()` converts YAML rules to canonical module/seam IDs and
removes all language options. `policy.Evaluate(snapshot, policy)` returns sorted
per-relation decisions, actionable violations and analyzer diagnostics. Invalid
IR/policy returns an error. Result exit semantics are 0 for clean, 1 for
architecture violations and 2 for incomplete analysis; callers must handle API
errors as errors too.

Rules are directed consumer/provider grants. Explicit deny wins across matching
module and seam rules regardless of input order. A member in multiple seams is
denied if any matching seam rule denies it. Exact duplicate/conflicting rule
pairs are invalid configuration.

A module grant allows dependency intent only. A seam grant additionally permits
references to its declared members and enables a dependency on its owning module.
A module import remains a dependency record; it does not prove that an interface
was used, a member was executed, or any behavior was tested. Adapters should emit
module import relations separately from member references. `seam-use` decisions
mean static reference to a declared seam/member, without data-flow guarantees.

Default allow permits dependencies and declared seam-member references for
incremental adoption; default deny requires grants. Neither permits a proven
bypass: a cross-module symbol outside all public seams or a relation with
`bypassesSeam: true` violates the boundary. Adapters must establish the latter
fact for deep/forbidden entrypoint paths; the core never guesses from source
extensions or import syntax. Intra-module relations, structural `implements` facts and `exposes` declarations
are not consumer access checks. All actual cross-module consumer edges remain
visible in snapshots and comparison.

Unresolved/dynamic references stay in diagnostics, with unresolved/dynamic
Evidence resolution. `unresolved: error` promotes uncertainty to an analysis error;
`warn` preserves it as an explicit warning. Existing error diagnostics are never
downgraded by relaxed policy. Evidence and source coordinates are retained on
violations; stable violation IDs exclude coordinates.

`snapshot.Compare(base, head)` compares canonical architecture entities by ID:
modules, seams, public symbols and cross-module relations. Added/removed consumer
relations, contract membership changes, and signature modifications appear as
changes. Signature edits retain symbol identity. Renames produce add/remove
records. Private symbol edits, internal relations, source line changes and analyzer
diagnostics do not pollute the architecture diff. Unconsumed symbols outside a declared seam are hidden even when language-exported.
Cross-module referenced targets remain architecture-relevant, including forbidden
private/unseamed references, so their exposed signatures and bypasses are visible.

`snapshot.ComparePolicies` also evaluates each revision with its own normalized
policy and reports added/removed violations. Incomplete strict analysis rejects
this comparison. This API does not resolve revisions, read Git, create worktrees,
collect coverage or import test evidence. JSON has schemaVersion 1, sorted
changes and before/after records; relation evidence is excluded from diff values.
Original snapshots and policy reports retain source evidence for inspection.

The Git layer reads exact commit blobs into isolated temporary directories,
never checking out or modifying the working tree. Uncommitted files are excluded.
Symlinks, submodules and unsupported tree entries fail explicitly when encountered;
Git archive export attributes do not affect the extracted contents. Git hooks are not run. Revision inspection uses the same extraction. The config
root currently needs to be the Git repository root for revision operations.
Server sessions cache complete analyzed graphs by commit, config hash and analysis
context. Caches are bounded, in memory, and cleared by restarting the server after
external tool/dependency changes. Contract evidence is re-evaluated separately.
The browser shows before/after identities and overlays module/edge changes.
