# SeamGraph roadmap

Build a local-first code review workspace with architecture context: understand
what changed, read the patch, trace its consequences, and record review findings
in one place. Build on the existing Git comparison, consumer references, policy
checks and contract evidence.

These items describe planned work, not implemented capabilities. The order below
is a proposed delivery sequence. Check existing support before implementing each
item and extend the shared IR and workflows where appropriate.

## 1. Realistic examples and end-to-end review scenarios

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

Build on the Code explorer tree, object details and source navigation to deliver
a review workflow without requiring a hosting account or network. Add comparison
and review state to those shared views rather than building a separate explorer.

- [ ] Select a target branch and review branch; show the resolved revisions and
  comparison mode throughout the review.
- [ ] Default to changes from the merge base to the review branch tip. Separately
  offer a comparison of the current target tip against the review branch tip;
  explain that this compares the two trees and is not a simulated merge result.
- [ ] Handle missing refs, unavailable history and ambiguous merge bases explicitly.
- [ ] Extend the explorer tree with added, modified and removed files and objects;
  connect selections to diffs and before/after relationship evidence. Keep removed
  objects inspectable and distinguish uncertain object matching.
- [ ] Show commits and changed files, grouped by module and seam where attribution
  is available. Keep unassigned files and root configuration changes visible.
- [ ] Provide unified and side-by-side diffs, file navigation and search, including
  added, deleted and renamed files. Clearly identify binary or unavailable diffs.
- [ ] Preserve the existing ability to compare working changes, clearly separated
  from committed branch changes.
- [ ] Support viewed markers and a review checklist, persisted locally per
  repository and comparison.

Acceptance: review an unpublished local branch from summary to individual hunks
without changing the checkout or index. Comparison labels make the chosen base
and head unambiguous, and removed code remains inspectable.

### Branch hotspots and commit analysis

Show where a branch's work is concentrated and how its commit history describes
that work. Connect hotspot summaries to the code explorer, diffs and architecture
context so users can investigate the underlying changes.

- [ ] Analyze commits reachable from the review branch but not the selected
  target, showing the resolved revisions, history range and merge handling policy.
  Distinguish commit-history activity from the final net tree difference.
- [ ] Show per-file change frequency as the number of analyzed commits touching
  each file, alongside lines added, lines deleted and cumulative churn. Count
  churn as additions plus deletions, not unique lines or net growth.
- [ ] Show final net additions and deletions separately from cumulative churn so
  repeated edits and reverted work remain understandable. Optionally show change
  size relative to file size with an explicit denominator; do not label churn as
  a percentage of unique code changed.
- [ ] Define and expose merge counting and rename attribution. Avoid silently
  double-counting merged changes; distinguish detected renames from uncertain
  edited moves. Mark binary files and unavailable line statistics explicitly.
- [ ] Provide a sortable hotspot table and a visual view, such as a directory
  treemap colored by change frequency or churn. Let users switch metrics, filter
  paths and exclude generated or vendored content with visible filter settings.
- [ ] Drill from a hotspot into its touching commits, individual diffs and final
  branch diff. Aggregate by directory and declared module where available, keeping
  unassigned files visible.
- [ ] Parse Conventional Commit subjects into type, optional scope and breaking
  change markers; also recognize BREAKING CHANGE footers. Preserve original
  messages and group unmatched commits explicitly rather than guessing their type.
- [ ] Show commit counts by type and scope, a timeline grouped by type, and
  breaking changes. Support custom types and clarify whether charts count commits,
  touched files or line churn; these measures are not interchangeable.
- [ ] Make chart segments actionable filters for the commit list and hotspots.
  Keep reverts, merge commits and squash-history limitations visible when relevant.
- [ ] Include hotspot and commit summaries in review exports with their range,
  metric definitions and filters. Present activity as review guidance, without
  treating churn or commit type as proof of defects, complexity or developer
  productivity.

Acceptance: a branch with repeated edits, a revert, a rename and mixed commit
formats produces explainable frequency, churn and net-diff totals. Users can
inspect the commits behind every hotspot or chart group, and unmatched messages
and unavailable statistics remain visible.

## 4. Change impact and architecture context

Make “explain this change in context” the central review interaction.

- [ ] Navigate from a changed symbol to its seams, direct consumers, transitive
  dependents, boundary violations and mapped contract tests.
- [ ] Navigate from an architecture change or finding to the relevant source diff
  and before/after reference evidence.
- [ ] Show whether consumers changed alongside a changed seam.
- [ ] Distinguish observed references from possible downstream impact; expose
  incomplete analysis and uncertain symbol attribution.
- [ ] Show changed seams with stale evidence, requirements without mapped tests,
  and missing evidence for declared consumer contract obligations.
- [ ] Link contract gaps to supporting evidence and relevant test commands. Keep
  execution coverage separate from behavioral verification.

Acceptance: a changed seam can be traced to its observed consumers and available
contract evidence on the appropriate revision, without presenting inferred impact
or passing tests as proof of complete verification.

## 5. Review notes and export

- [ ] Anchor local notes to diff locations or architecture objects, recording the
  revision and enough context to identify their original subject.
- [ ] Track unresolved and resolved questions. Flag notes and viewed markers for
  reconsideration when their underlying code changes; preserve original context
  when an anchor can no longer be located.
- [ ] Export a concise Markdown review report containing changed seams, affected
  consumers, new and resolved violations, stale contract evidence, findings and
  outstanding questions.
- [ ] Include comparison revisions, source locations and investigation commands
  so findings can be reproduced.
- [ ] Export a self-contained HTML architecture/review snapshot for onboarding,
  design reviews and sharing. Make included source content explicit at export.

Acceptance: a review survives restarting the explorer and refreshing the branch;
exported findings retain their evidence and comparison identity.

## 6. Guided architecture setup and incremental adoption

- [ ] Suggest module boundaries from repository structure and existing
  dependencies, with a graph and configuration preview before applying changes.
- [ ] Keep inferred boundaries as reviewable suggestions; require explicit seam
  declarations rather than treating imports as proof of interface use.
- [ ] Support a reviewed violation baseline so existing debt can be adopted while
  new violations fail checks.
- [ ] Give exceptions a reason, owner and optional expiry; surface expired entries
  and baseline entries whose violations have been resolved.

Acceptance: a repository can adopt useful checks incrementally, with explicit
configuration changes and visible existing debt.

## 7. Architecture investigation

- [ ] Explain why one module depends on another using concrete references.
- [ ] Show shortest dependency paths with reference-kind filters.
- [ ] Explain cycles and let users inspect the references forming each cycle.
- [ ] Provide reverse-consumer search and focused views for larger repositories.

Acceptance: users can investigate a dependency or cycle without manually tracing
arrows across the entire graph.

## 8. GitHub pull request integration

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

Extend the existing contract-requirement model to connect functional and
nonfunctional requirements, architectural decisions and revision-specific
verification evidence. Keep requirements (what must hold) distinct from ADRs
(why a decision was made), with explicit links between them.

- [ ] Define a versioned Markdown format with structured metadata for stable
  requirement IDs, kind, title, scope, linked ADRs and verification methods, plus
  prose rationale and acceptance criteria. Validate references and report missing
  or ambiguous IDs.
- [ ] Reuse existing contract requirements and test mappings rather than creating
  a parallel requirement registry; define compatibility and migration behavior.
- [ ] Link requirements to explicit architecture rules, reviewed behavioral test
  mappings, benchmark reports or recorded human reviews as appropriate.
- [ ] Record evidence provenance, assessed revision and relevant configuration.
  Benchmark evidence must include its workload, environment and thresholds.
- [ ] Report passing checks, failures, stale evidence, missing evidence and human
  review requirements separately. Related source code or a passing mapped test
  must not imply that an entire requirement has been satisfied.
- [ ] In branch review, show affected requirements and linked ADRs alongside the
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
