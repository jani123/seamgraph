# SeamGraph roadmap

Build a local-first code review workspace with architecture context: understand
what changed, read the patch, trace its consequences, and record review findings
in one place. Build on the existing Git comparison, consumer references, policy
checks and contract evidence.

This roadmap includes implemented and planned work. The order below is a proposed
delivery sequence. Check existing support before implementing each item and
extend the shared IR and workflows where appropriate.

Audit updated 2026-10-27 against implementation, automated acceptance tests and
local HTTP probes. Checked items meet the complete bullet; unchecked items marked
**Partial** have working foundations but still need the listed behavior. Other
unchecked items remain planned. A passing test suite verifies covered scenarios,
not completion of every roadmap acceptance criterion.

| Part | Audited state | Main remaining work |
| --- | --- | --- |
| 1. Examples | Implemented and tested | Extend scenarios as the remaining features land |
| 2. Code explorer | Implemented and tested | Preserve adapter limits and shared navigation |
| 3. Local branch review | Implemented and tested | Exercise the complete branch workflow and preserve explicit history/matching limits |
| 4. Change impact | Partial | Connect revision-specific source, consumers, violations and contract gaps in the review UI |
| 5. Notes and export | Partial | Object anchors, comment resolution, concise reports and reproducible commands |
| 6. Guided setup | Partial | Review boundary drafts before writing; baseline/exception lifecycle |
| 7. Investigation | Partial | Shortest paths, explicit cycle investigation and stronger reverse-consumer search |
| 8. GitHub PR integration | Planned | PR metadata, threads, checks and deliberate publication |
| 9. Requirements and ADRs | Contract foundation implemented | Structured Markdown, ADR links, broader verification evidence and dedicated skill |
| 10. Documentation review | Planned | Dedicated authoring/review workflow, drift detection and skill |

Part 3 implementation update:

- History-only hotspots now open a touching commit; the generated temporary-file
  regression covers navigation even when neither endpoint contains the path.
- Local review tools and exports now work without `seamgraph.yaml`. Unavailable
  architecture context stays explicit instead of disabling file review/history.
- The shared Compare changes controls offer branch mode with a unique merge
  base and a separate target-tip comparison; there is no separate branch form.
  Branch modes use populated target/review dropdowns, mark the current branch,
  suggest a known target and keep manual ref entry under Revisions (advanced). Exact revision and working-tree comparisons remain available.
- Unified/split diffs, base/head reference evidence, a saved checklist, activity
  sorting/grouping/exclusions, a heatmap and type/scope charts share Code navigation.

Remaining gap in part 4: the Impact panel still chooses one side's architecture
references without its own explicit revision control. The Code symbol details now
provide separate base/head reference views, but the deeper Impact workflow remains
unfinished as documented below.

Validation artifacts for this audit live under the ignored `.build/` directory:
`roadmap-audit-check.log`, `roadmap-audit/probes.json` and
`roadmap-audit/unconfigured.json` and `roadmap-audit/baseline-probe.json`.
Permanent acceptance coverage is linked in the
section notes below. Part 3 validation is recorded in `.build/part3-check.log`;
desktop/mobile screenshots are `.build/branch-review-desktop.png` and
`.build/branch-review-mobile.png`.

## 1. Realistic examples and end-to-end review scenarios

**Audit: implemented.** `scripts/examples/test_scenarios.py` verifies application
flows, authored graph facts, history totals, stale/failing/missing evidence,
working-tree isolation and a bounded 138-module case. The fulfillment browser
scenarios exercise navigation, review persistence, exports and responsive views.
Intentional incompatible-consumer and documentation defects remain labeled
fixtures, rather than claims that those defects are automatically diagnosed.

Use a small order fulfillment application as the shared example. Its main flow is
place order → reserve stock → authorize payment → fulfill, with cancellation and
failure handling. Use local payment and shipping fakes and deterministic data;
no real accounts or external services are required.

- TypeScript storefront: cart, checkout and order status.
- Go order service: order lifecycle and coordination.
- Go inventory module: stock reservations.
- Go payment adapter: simulated authorization and refunds.
- Python fulfillment worker: shipment processing.
- Python reporting: order and fulfillment summaries.

Start with the Go order/inventory core and TypeScript client, then add the Python
worker and reporting. Keep the example bounded rather than building a complete
commerce platform. Make HTTP and message contracts explicit, without implying
that cross-language runtime relationships are automatically traced by SeamGraph.

Establish richer example repositories and reproducible histories before building
the other roadmap features, then extend their scenarios as those features land.
Use them as the shared foundation for testing realistic code and branch changes. Keep focused
fixtures for isolated checks; use these examples to verify complete workflows
and provide useful repositories for manual exploration.

- [x] Create a coherent example application with nested directories, multiple
  modules, public seams, internal implementation details and meaningful consumer
  relationships. Cover Go, TypeScript and Python, including their supported
  symbol kinds and known analysis limits.
- [x] Include branching calls, shared consumers, dependency cycles, long dependency
  paths, similarly named objects and files outside declared modules. Provide
  enough structure to exercise tree navigation, search and focused graphs.
- [x] Generate deterministic local Git histories with a base branch, review
  branches and a diverging target. Include repeated edits, reverts, file additions
  and deletions, exact and edited moves, symbol/signature changes, binary files
  and merge commits. Include staged, unstaged and untracked working changes in
  dedicated scenarios.
- [x] Mix Conventional Commit types, scopes, breaking subject markers and footers,
  custom types and unmatched messages. Record expected commit groups, per-file
  touch counts, cumulative churn and final net diffs independently of the UI.
- [x] Include intentional boundary violations, resolved violations, changed
  consumers and contract requirements with passing, failing, missing and stale
  evidence. Ensure test results and revision identity can be reproduced.
- [x] Add concise overview documentation, linked ADRs and requirements, plus
  deliberate stale claims, broken references and overlapping prose for
  documentation review. Label intentional defects in a scenario guide.
- [x] Document each scenario's review questions and expected findings, including
  uncertainty and unsupported cases. Assert meaningful outcomes rather than
  merely snapshotting generated output.
- [x] Exercise shared workflows end to end: directory exploration, object details,
  branch comparison, hotspot drill-down, architecture impact, review persistence
  after another commit and evidence-preserving exports as features land.
  The Review workspace exercises directory/file navigation, adapter object details,
  hotspot commit/patch drill-down, observed references and possible impact,
  file notes/viewed state across a new commit and server restart, and JSON,
  Markdown and self-contained HTML exports with evidence and original anchors.
- [x] Provide a bounded larger scenario for navigation and rendering performance,
  with explicit size and performance expectations. Keep generation reproducible,
  local and independent of remote hosting, credentials or runtime downloads.

Acceptance: developers can create and open the examples with documented commands
and work through realistic review scenarios. Automated checks verify the expected
facts and transitions, including history edge cases, while the same examples
support manual evaluation of clarity and usability.

### Order fulfillment review scenarios

- [x] Add cancellation across orders, inventory, payments and fulfillment, with
  requirements for releasing stock and preventing duplicate charges or refunds.
- [x] Change a payment seam and deliberately leave one consumer incompatible.
- [x] Introduce an inventory boundary violation and provide a follow-up fix.
- [x] Repeatedly revise checkout to create predictable frequency and churn hotspots.
- [x] Move files, revert a change and merge a diverging target, with independently
  recorded expected history and final-diff results.
- [x] Change behavior while leaving mapped test evidence or documentation stale,
  then provide a branch that resolves the gaps.

Implemented examples and expected findings: [order fulfillment](examples/order-fulfillment/README.md)
and [scenario guide](examples/order-fulfillment/SCENARIOS.md). Run `task examples:test`;
the required `task check` also exercises the generated repository in the browser.

## 2. Code explorer

**Audit: implemented.** `internal/bootstrap/code_test.go` covers all three
languages without architecture configuration, declared identities, body changes
and removed-object source/relationship revisions. Browser tests cover tree
navigation, filters, relationships, unified diffs and source readability during
analysis failures. Focused relationships are evidence lists; a separate focused
code graph is optional in this section.

Explore what is inside a directory without first declaring modules or seams.
Use a tree as the primary view, with object details and optional focused graphs
for investigating relationships.

- [x] Browse directories, files and top-level symbols in a compact expandable
  tree. Expand methods and members on demand rather than exposing every detail
  immediately.
- [x] Search objects and filter by symbol kind, visibility and changed status,
  preserving enough directory context to locate results.
- [x] Show the selected object's signature, source location, available
  documentation and relevant reference evidence in a details panel.
- [x] Offer focused relationship views for callers, consumers and type
  relationships where supported by the language adapter. Expose analysis limits
  and distinguish observed references from inferred relationships.
- [x] Show declared module and seam membership as additional context, while
  keeping directory exploration usable without architecture configuration.
- [x] Reuse shared symbol identities and adapter facts where available; extend
  analysis explicitly where the existing architecture-oriented data omits objects.
- [x] Provide reusable tree navigation, object selection, source viewing and
  revision-aware object identities for the later branch review workflow.

Acceptance: a user can select a directory, understand its contents, inspect an
object and follow its supported relationships without defining architecture
boundaries. The default view stays compact, and browsing components can be reused
by branch review without a separate navigation model.

## 3. Local branch review

**Implemented and tested.** Branch selection defaults to a unique
merge base and separately offers target-tip comparison. The resolved target,
compared base, review tip and mode remain visible. Missing refs, shallow history,
unrelated histories and multiple merge bases are explicit errors. Exact revision
and working-change scopes remain separate. Reading a review never checks out code
or changes the index.

Code provides changed/removed objects, exact-content file moves, unified and
side-by-side diffs, diff search, file navigation and separately selectable base/head
reference evidence. Stable IDs match objects; changed IDs and edited moves are
explicitly uncertain. File review groups changed files by module/seam context,
keeps root configuration/unassigned paths, and saves viewed markers and a custom
checklist per comparison mode/ref pair. Revised checklists need reconsideration.
Architecture configuration is optional; unavailable architecture remains labeled.

`internal/gitworkspace/review_test.go` exercises diverging/criss-cross/unrelated
and shallow histories without checkout/index mutation.
`internal/bootstrap/branch_review_test.go` covers configuration-free review, exact
moves, restart persistence, stale saves and isolation between modes. The local
branch browser scenario covers the integrated workflow, including history-only
paths, filtering, split source and exports at desktop/mobile sizes.

Build on the Code explorer tree, object details and source navigation to deliver
a review workflow without requiring a hosting account or network. Add comparison
and review state to those shared views rather than building a separate explorer.

- [x] Select a target branch and review branch; show the resolved revisions and
  comparison mode throughout the review.
- [x] Default to changes from the merge base to the review branch tip. Separately
  offer a comparison of the current target tip against the review branch tip;
  explain that this compares the two trees and is not a simulated merge result.
- [x] Handle missing refs, unavailable history and ambiguous merge bases explicitly.
- [x] Extend the explorer tree with added, modified and removed files and objects;
  connect selections to diffs and before/after relationship evidence. Keep removed
  objects inspectable and distinguish uncertain object matching.
- [x] Show commits and changed files, grouped by module and seam where attribution
  is available. Keep unassigned files and root configuration changes visible.
- [x] Provide unified and side-by-side diffs, file navigation and search, including
  added, deleted and renamed files. Clearly identify binary or unavailable diffs.
- [x] Preserve the existing ability to compare working changes, clearly separated
  from committed branch changes.
- [x] Support viewed markers and a review checklist, persisted locally per
  repository and comparison.

Acceptance: review an unpublished local branch from summary to individual hunks
without changing the checkout or index. Comparison labels make the chosen base
and head unambiguous, and removed code remains inspectable.

### Branch hotspots and commit analysis

**Implemented and tested.** History is target..review in branch
mode, independently of the merge-base..review net diff. Non-merge commits are
counted once and paths are not followed across renames. The UI exposes that policy,
uncertain edited moves, unknown binary statistics, reverts and squash limitations.
Frequency, cumulative additions/deletions/churn and final net totals stay separate.

Hotspots provide metric selection, sorting, path filters, visible generated/vendor
path exclusions, file/directory/module grouping and an actionable heatmap.
Grouped frequency counts distinct touching commits, rather than summing per-file
touches. Type/scope charts count commits and filter History and hotspot paths;
hotspot totals retain the full history range for matching paths. History includes
original messages, breaking markers, a timeline by type and affected-file links
to each commit's exact parent/head source. History-only paths remain inspectable.
JSON/Markdown/self-contained HTML preserve raw facts and the selected summary,
comparison identity, metric definitions and filters.

The independent generated-history oracle and fulfillment browser scenario verify
frequency/churn/net totals, reverts, moves, merges, binary files and custom commit
types. Application summary tests cover grouped distinct-commit frequency and
filter semantics; the new local branch browser scenario covers shared filters,
heatmap, touching commits and exports without architecture configuration.

Show where a branch's work is concentrated and how its commit history describes
that work. Connect hotspot summaries to the code explorer, diffs and architecture
context so users can investigate the underlying changes.

- [x] Analyze commits reachable from the review branch but not the selected
  target, showing the resolved revisions, history range and merge handling policy.
  Distinguish commit-history activity from the final net tree difference.
- [x] Show per-file change frequency as the number of analyzed commits touching
  each file, alongside lines added, lines deleted and cumulative churn. Count
  churn as additions plus deletions, not unique lines or net growth.
- [x] Show final net additions and deletions separately from cumulative churn so
  repeated edits and reverted work remain understandable. Optionally show change
  size relative to file size with an explicit denominator; do not label churn as
  a percentage of unique code changed.
- [x] Define and expose merge counting and rename attribution. Avoid silently
  double-counting merged changes; distinguish detected renames from uncertain
  edited moves. Mark binary files and unavailable line statistics explicitly.
- [x] Provide a sortable hotspot table and a visual view, such as a directory
  treemap colored by change frequency or churn. Let users switch metrics, filter
  paths and exclude generated or vendored content with visible filter settings.
- [x] Drill from a hotspot into its touching commits, individual diffs and final
  branch diff. Aggregate by directory and declared module where available, keeping
  unassigned files visible.
- [x] Parse Conventional Commit subjects into type, optional scope and breaking
  change markers; also recognize BREAKING CHANGE footers. Preserve original
  messages and group unmatched commits explicitly rather than guessing their type.
- [x] Show commit counts by type and scope, a timeline grouped by type, and
  breaking changes. Support custom types and clarify whether charts count commits,
  touched files or line churn; these measures are not interchangeable.
- [x] Make chart segments actionable filters for the commit list and hotspots.
  Keep reverts, merge commits and squash-history limitations visible when relevant.
- [x] Include hotspot and commit summaries in review exports with their range,
  metric definitions and filters. Present activity as review guidance, without
  treating churn or commit type as proof of defects, complexity or developer
  productivity.

Acceptance: a branch with repeated edits, a revert, a rename and mixed commit
formats produces explainable frequency, churn and net-diff totals. Users can
inspect the commits behind every hotspot or chart group, and unmatched messages
and unavailable statistics remain visible.

## 4. Change impact and architecture context

**Audit: partial.** Architecture comparison includes before/after symbols,
references, policy violations, requirement edits and verification status changes.
The Code Impact tab offers changed symbol links, observed architecture references,
possible transitive module impact and complete comparison JSON. Module inspection
shows seams, consumers, mapped tests, requirement coverage and evidence status.
`internal/snapshot/diff_test.go`, verification tests and example scenarios cover
these facts; the fulfillment browser review exercises impact for a selected
orders symbol.

Remaining: connect these facts into a revision-labeled review flow from changed
symbols to seams, consumer diffs, violations and mapped tests, and back to source.
Show whether individual consumers changed with a seam. Contract differences are
available as comparison data, but there is no dedicated review gap list with
supporting evidence and runnable commands. The possible-impact closure is over
modules and must remain separate from observed symbol references. Adapter
uncertainty is surfaced through diagnostics. Impact still needs to carry Code
matching and revision labels through the architecture evidence.

Make “explain this change in context” the central review interaction.

- [ ] **Partial:** Navigate from a changed symbol to its seams, direct consumers, transitive
  dependents, boundary violations and mapped contract tests.
- [ ] **Partial:** Navigate from an architecture change or finding to the relevant source diff
  and before/after reference evidence.
- [ ] Show whether consumers changed alongside a changed seam.
- [ ] **Partial:** Distinguish observed references from possible downstream impact; expose
  incomplete analysis and uncertain symbol attribution.
- [ ] **Partial:** Show changed seams with stale evidence, requirements without mapped tests,
  and missing evidence for declared consumer contract obligations.
- [ ] **Partial:** Link contract gaps to supporting evidence and relevant test commands. Keep
  execution coverage separate from behavioral verification.

Acceptance: a changed seam can be traced to its observed consumers and available
contract evidence on the appropriate revision, without presenting inferred impact
or passing tests as proof of complete verification.

## 5. Review notes and export

**Audit: partial.** File and base/head line comments preserve revision, side,
line, digest and bounded original context in `.seamgraph/reviews.jsonl`. File notes
have viewed/resolved controls. Changed file digests mark notes/comments for
reconsideration; unavailable anchors preserve context. Comments and notes survive
restart, and stale writes are rejected. Both diff layouts indicate comments and
allow editing in place without leaving the current file, layout, filters or scroll
position. Edits retain original anchors and reject conflicting versions under
the review-store lock. Application/bootstrap tests and the
fulfillment/comment browser scenarios verify these transitions.

Remaining: architecture-object anchors; resolving individual line
comments; a concise
architecture/evidence Markdown report with investigation commands; and a more
readable shareable snapshot. The collected-comments Markdown report is readable
and comparison-wide. Full review Markdown and self-contained HTML now add an
activity summary before the complete JSON evidence, with source inclusion disclosed. Those existing
formats preserve evidence but do not satisfy the complete report workflow.

- [ ] **Partial:** Anchor local notes to diff locations or architecture objects, recording the
  revision and enough context to identify their original subject.
- [ ] **Partial:** Track unresolved and resolved questions. Flag notes and viewed markers for
  reconsideration when their underlying code changes; preserve original context
  when an anchor can no longer be located.
- [ ] **Partial:** Export a concise Markdown review report containing changed seams, affected
  consumers, new and resolved violations, stale contract evidence, findings and
  outstanding questions.
- [ ] **Partial:** Include comparison revisions, source locations and investigation commands
  so findings can be reproduced.
- [ ] **Partial:** Export a self-contained HTML architecture/review snapshot for onboarding,
  design reviews and sharing. Make included source content explicit at export.

Acceptance: a review survives restarting the explorer and refreshing the branch;
exported findings retain their evidence and comparison identity.

## 6. Guided architecture setup and incremental adoption

**Audit: partial.** `internal/projectfiles/discover.go` drafts modules from
language/source directories and deliberately creates no inferred seams. Opening
a remote workspace can write that exploratory configuration before review;
`init` uses a fixed starter. Neither is the requested dependency-informed preview
and approval workflow. Existing rule editing does provide preview, confirmation
and stale-write protection, tested in HTTP and browser scenarios.

`check --baseline` accepts a reviewed JSON list of stable violation IDs and
preserves analysis errors. `internal/report/report_test.go` covers error
preservation; an audit CLI probe confirmed that an accepted violation stops
failing while an unrelated baseline leaves the violation failing. This meets the
baseline enforcement bullet. Baseline authoring/review remains a manual process.
Policy rules do not yet carry exception reasons, owners or expiry, and resolved
baseline entries are not reported.

- [ ] **Partial:** Suggest module boundaries from repository structure and existing
  dependencies, with a graph and configuration preview before applying changes.
- [ ] **Partial:** Keep inferred boundaries as reviewable suggestions; require explicit seam
  declarations rather than treating imports as proof of interface use.
- [x] Support a reviewed violation baseline so existing debt can be adopted while
  new violations fail checks.
- [ ] Give exceptions a reason, owner and optional expiry; surface expired entries
  and baseline entries whose violations have been resolved.

Acceptance: a repository can adopt useful checks incrementally, with explicit
configuration changes and visible existing debt.

## 7. Architecture investigation

**Audit: partial.** Selecting a module/edge exposes concrete reference evidence;
CLI `inspect` and Code consumers/callers offer reusable investigation paths.
Browser tests verify dependency-kind filtering, seam expansion, neighborhood
focus and a 100-module/500-edge graph. The layout groups strongly connected
components, but this is not an explicit cycle explanation feature.

Remaining: a shortest dependency path query with reference-kind filters, cycle
listing and evidence drill-down, and a dedicated reverse-consumer search across
larger repositories. Existing consumers lists and neighborhood focus provide
part of the last item.

- [x] Explain why one module depends on another using concrete references.
- [ ] Show shortest dependency paths with reference-kind filters.
- [ ] Explain cycles and let users inspect the references forming each cycle.
- [ ] **Partial:** Provide reverse-consumer search and focused views for larger repositories.

Acceptance: users can investigate a dependency or cycle without manually tracing
arrows across the entire graph.

## 8. GitHub pull request integration

**Audit: planned.** Remote Git cloning/opening is implemented and tested in
`internal/gitworkspace/remote_test.go`, but does not fetch PR descriptions,
discussion threads, checks or review state. There is no PR review submission
workflow. Existing local review is the foundation, subject to the gaps in part 3.

Add hosting integration after the local review workflow is useful on its own.

- [ ] Open a pull request into the same review workspace, including its description,
  base/head revisions, commits, discussion threads and check results.
- [ ] Associate local findings with existing threads and flag outdated context
  after new pushes.
- [ ] Preview and explicitly submit selected comments or a review to GitHub.
- [ ] Keep local review available for unpublished branches and repositories on
  other hosts; surface remote failures without losing local review state.

Acceptance: users can read PR context and deliberately publish a review while
retaining the same architecture and evidence navigation available locally.

## 9. Requirements and ADR verification

**Audit: partial foundation.** `seamgraph-contracts.yaml` already supplies a
versioned, strictly validated requirement/test-mapping registry. Verification
checks explicit reviewed mappings and revision/fingerprint provenance, separates
execution coverage from behavioral verification, and distinguishes missing,
failed, stale, partial and unknown evidence. Comparison reports requirement and
verification changes. `internal/verification/verification_test.go`, runner tests
and example scenarios exercise these paths.

Remaining: the proposed structured Markdown format and compatibility rules,
validated ADR links, broader scopes and architectural-rule mappings, benchmark
workload/environment/threshold evidence, human review evidence, an integrated
requirements/ADR view in branch review, and a dedicated drafting/investigation
skill. Existing ADR Markdown files and the general SeamGraph agent skill do not
implement that workflow.

Extend the existing contract-requirement model to connect functional and
nonfunctional requirements, architectural decisions and revision-specific
verification evidence. Keep requirements (what must hold) distinct from ADRs
(why a decision was made), with explicit links between them.

- [ ] Define a versioned Markdown format with structured metadata for stable
  requirement IDs, kind, title, scope, linked ADRs and verification methods, plus
  prose rationale and acceptance criteria. Validate references and report missing
  or ambiguous IDs.
- [ ] **Partial:** Reuse existing contract requirements and test mappings rather than creating
  a parallel requirement registry; define compatibility and migration behavior.
- [ ] **Partial:** Link requirements to explicit architecture rules, reviewed behavioral test
  mappings, benchmark reports or recorded human reviews as appropriate.
- [ ] **Partial:** Record evidence provenance, assessed revision and relevant configuration.
  Benchmark evidence must include its workload, environment and thresholds.
- [ ] **Partial:** Report passing checks, failures, stale evidence, missing evidence and human
  review requirements separately. Related source code or a passing mapped test
  must not imply that an entire requirement has been satisfied.
- [ ] **Partial:** In branch review, show affected requirements and linked ADRs alongside the
  source changes, check results and evidence that needs renewal. Distinguish
  declared links from inferred impact and expose incomplete analysis.
- [ ] Provide an agent skill to draft requirement documents, propose ADR links,
  scopes and verification mappings, and investigate evidence gaps. Keep proposed
  declarations reviewable; SeamGraph owns validation and repeatable checks.

Acceptance: a documented requirement can be traced through its linked decision,
scope and verification evidence to the assessed source revision. Branch review
shows which checks pass and which evidence is stale or missing, without treating
agent interpretation or partial checks as proof of full requirement satisfaction.

## 10. Documentation clarity and review

**Audit: planned.** Human-facing guides, ADRs and the general architecture agent
skill exist, and examples deliberately contain stale claims, broken references
and overlapping prose. Scenario tests verify those fixtures exist or are fixed;
they do not perform documentation review. There is no dedicated authoring/review
skill, documentation link/overlap analysis or branch-aware drift workflow.
The guidance below remains the intended standard for that future feature.

Help readers understand the system at a high level: its purpose, main parts, how
they connect, and the decisions and constraints that matter. Preserve knowledge
that is difficult to recover from source code or reliable commands. Add detail
when it helps readers act or avoid a misunderstanding.

- [ ] Provide agent authoring guidance that leads with the high-level picture and
  documents intent, rationale, tradeoffs, requirements, external promises,
  operational knowledge and important limitations.
- [ ] Discourage function inventories, line-by-line implementation narration,
  duplicate configuration references and unnecessary documents. Prefer links to
  authoritative sources and generate reference material when useful.
- [ ] Review documentation for overlapping content, broken references and stale
  examples, with concrete evidence supporting each finding. Account for documents
  that intentionally serve different audiences.
- [ ] In branch review, identify changes that may invalidate documented behavior,
  architecture overviews, setup instructions or operational procedures. Distinguish
  verified inconsistencies from suggestions requiring editorial judgment.
- [ ] Provide an agent skill to assess whether content adds useful knowledge,
  propose concise revisions or consolidation, and preserve rationale and necessary
  audience-specific guidance. Keep edits reviewable rather than automatically
  deleting apparent duplication.
- [ ] Keep review output focused on consequential documentation gaps and drift;
  avoid requiring prose for every source change or generating documentation merely
  to satisfy coverage targets.

Acceptance: readers can quickly understand the system and locate deeper references.
Documentation review explains which content may be stale, redundant or missing and
why, without expanding the documentation into a restatement of the source code.
