# ADR 0002: Shared policy and comparison cores before adapters

Status: accepted implementation direction, 2026-10-16.

The user confirmed literal, non-overlapping module roots for v1 and approved
implementing minimal policy and comparison cores before adapters. A root owns its
whole subtree. Separately declared modules require disjoint roots; there is no
implicit nested ownership or glob precedence.

Implement normalized policy input using canonical IR identities, independent of
language-specific YAML module options. Config compiles declarations to this input.
Evaluate observed cross-owner imports separately from seam-member use. A seam
grant can permit importing its provider but an import never proves member use.
Module grants permit dependencies, not access to arbitrary symbols. Explicit deny
wins over allow; matching seam denies beat matching grants, including for members
listed in more than one seam. Default allow permits dependency/seam use for
incremental adoption but never silently permits a proven seam bypass. Intra-module
relations and exposure declarations are not consumer edges.

Adapters must normalize forbidden entrypoint/deep-path access into the optional
IR relation boolean `bypassesSeam`; this fact is not inferred from source paths by
policy. Nonmember symbol references are also bypasses. This additive schema-v1
field is introduced before any adapters or released snapshots exist; old fixtures
retain identical bytes because false is omitted. Partial merger conflicts include
this field. Any unresolved/dynamic diagnostic evidence follows the normalized
error/warn policy; error diagnostics always prevent clean analysis.

Compare validated canonical snapshots by stable identity. Report module/seam,
public symbol and observed cross-module consumer relation changes. Changed public
signatures are modifications to the same symbol, not additions. Ignore source
location changes, private-symbol changes, same-module relations and diagnostics
in architecture changes. Report full private/internal data only in original
snapshots. Renames remain explicit add/remove identities. Diff output is versioned
and sorted, and may compare normalized policy violations to report regressions.

This is not P5/P7 completion: CLI check/SARIF/baselines, Git ref isolation/cache,
web overlays, and contract verification remain in their scheduled phases.

Reconciliation during adapter integration: the same endpoints can have both a
permitted import and a deep import at different source locations. Their merged
relation preserves `bypassesSeam` with OR and retains precise `bypassEvidence`.
Different access paths are not contradictory facts. Conflicting endpoints/kinds
still fail validation, and permitted locations are not cited as bypass evidence.

Default comparison excludes unconsumed symbols outside declared seams, including
language-exported implementation helpers. A language export does not create an
architectural contract. Declared members and actual cross-module referenced
targets remain visible, including signatures exposed through a bypass.
