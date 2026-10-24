# Review scenarios and expected findings

Generate a new repository using the [overview commands](README.md). All branches
are local. `main` is the seed; `target` diverges by adding only `docs/target.md`.
The generator records exact revisions and independently authored tree-delta
expectations in `.seamgraph/scenarios.json`. That ignored directory also holds
local evidence and exports, so generating metadata does not invalidate evidence.
Commit hashes are reproducible for the same template and generator. Execution
run IDs and timestamps are intentionally real, not deterministic fake evidence.

| Branch | Review question | Expected finding |
| --- | --- | --- |
| `main` | Which public seams coordinate an order? | Orders consumes inventory reservations and payment authorization. HTTP and demo share orders. Checkout and status share the client. Reporting and operations share shipping. No cross-language runtime edges are invented. |
| `review/cancellation` | Can cancellation duplicate a refund, leak stock or ship later? | New Go Cancel method, HTTP cancellation route and TS cancellation consumer; Go assertions verify repeated cancellation, retry, missing/fulfilled orders and released stock. Python tombstones reject later ready messages and make cancellation repeatable. This does not prove real parcel interception or distributed ordering. |
| `review/payment-breaking` | Did all consumers adapt to the payment signature? | Authorize changes from int to int64 while orders still passes int. Go compilation and analysis fail explicitly (exit 2), so no complete architecture diff is claimed. |
| `review/payment-fixed` | Is the changed seam visible once the consumer compiles? | Orders converts the amount, the graph is analyzable and Authorize is a modified symbol. The orders source change is visible in the comparison file list. |
| `review/boundary` | Did debugging bypass the reservation seam? | One orders → inventory.DebugAvailable violation with source evidence, despite the existing reservation grant. |
| `review/boundary-fixed` | Is the bypass actually removed? | The consumer uses Stock.Available. No boundary violations remain; comparison from the preceding branch reports a removed violation. |
| `review/history` | Is activity the same as the final tree diff? | Three checkout edits have three touches, three additions, three deletions and churn six; final checkout diff is empty. A temporary note is added then deleted. Exact/edited moves, symbol rename, binary addition and a target merge remain inspectable. |
| `review/stale` | Did changed behavior leave old tests and prose behind? | An order limit of 100000 cents is introduced without a mapped limit assertion. Prior evidence is stale. `docs/checkout.md` falsely accepts any positive amount and links to missing-retry.md; checkout-copy.md overlaps that claim. These defects are intentional. |
| `review/evidence-fixed` | Are the gaps resolved and evidence refreshed? | Over-limit and inclusive-limit assertions are explicitly mapped, the false claim/link are fixed and overlapping prose removed. Collect a new Go run to obtain verified evidence for declared requirements only. |
| `review/evidence-failed` | Does a recorded run expose the stock leak? | Payment rejection omits Release; the authored assertion fails and current evidence is failed. |
| `review/evidence-missing` | Is a declared requirement automatically verified? | An additional cancellation obligation has no mapping; after a passing Go run, orders is partial and contract enforcement fails. |

## Current workflows

Use graph search for `orders`, select its module, expand seams, inspect consumers
and follow source links. Search `status` to see similarly named objects. Search
`warehouse-042` in the larger example; only that module should remain visible.
Choose All references for isolated module search; a narrower reference view
hides nodes with no remaining matching edges. The CLI `scan` preserves raw facts; `inspect` returns source-bearing references.

```sh
.build/seamgraph inspect seam:orders.lifecycle --root .build/fulfillment --json
.build/seamgraph diff --root .build/fulfillment \
  --base main --head review/boundary --json
.build/seamgraph diff --root .build/fulfillment \
  --base review/boundary --head review/boundary-fixed --json
.build/seamgraph snapshot --root .build/fulfillment \
  --output .build/fulfillment/.seamgraph/snapshot.json
```

In the browser choose Revisions, enter those base/head names and compare. The
resolved hashes and added/removed violations should match the CLI. Comparisons
must leave checkout and index unchanged. Snapshot exports retain relation evidence. The Review workspace additionally
exports persisted findings and original anchors as JSON, Markdown and standalone HTML.

The current comparison uses the two selected trees. Before merging target, use
`git merge-base target <review-ref>` explicitly when you want a merge-base review;
never describe the two-tree diff as a simulated merge. `review/history` has already
merged target: its merge base with target is target itself. Its seed is still
`main`. The manifest records `beforeMerge` to explore the pre-merge divergence.

To exercise staged, unstaged and untracked changes separately:

```sh
python3 scripts/examples/generate.py .build/fulfillment-working --working
.build/seamgraph diff --root .build/fulfillment-working \
  --base main --head WORKTREE --json
```

Checkout's label is Stage order in the index and Unstaged order on disk; a new
unassigned review note is untracked. Working comparison includes the combined
working tree. Dedicated Git commands (`git diff --cached`, `git diff`) distinguish
the staged and unstaged portions. Committed comparisons must exclude all three.

## Contract evidence

Run these commands inside the generated repository (adjust the binary path if
using a destination elsewhere):

```sh
../../.build/seamgraph test --root . --runner go
../../.build/seamgraph check --root . --contracts --json
git switch review/stale
../../.build/seamgraph check --root . --contracts --json # stale, exit 2
git switch review/evidence-fixed
../../.build/seamgraph test --root . --runner go
../../.build/seamgraph check --root . --contracts --json # verified
```

Use the failed/missing branches to repeat collection and inspection. A failing
collector returns exit 2 but retains actual failed outcomes; contract checking
then returns exit 1. Evidence is bound to the exact revision and input fingerprint.
Passing execution verifies only reviewed mapped assertions; member execution
coverage remains unknown. Python and TS run real behavior checks in the suite;
this example's SeamGraph evidence manifest maps Go tests only. No passing Go
result claims to verify the wire protocol, Python tombstones or entire CANCEL-1.

## History oracle

History activity is `target..review/history`, excluding merge commits and without
rename following. The merge's target-only note is excluded from activity, while
`main..review/history` final tree statistics include it. Churn means additions
plus deletions, not unique changed lines. Binary line statistics are unknown.
A rename counts deletion at the old path and addition at the new path for the
oracle; Git's separate `-M` view identifies exact and edited moves. SeamGraph
currently recognizes exact-content moves only and shows edited moves as add/delete.

Eight analyzed commits group as feat=1, fix=1, revert=1, chore=1, refactor=2,
custom assets=1 and unmatched=1. Scopes: checkout=2, fixtures=1, files=1,
status=1, brand=1, no scope=2. Two commits break contracts: one has a subject `!`,
one a BREAKING CHANGE footer. The merge subject is also unmatched but excluded
from activity metrics. Full original messages, revision IDs, per-commit deltas,
per-path touches/cumulative churn, final net changes and the counting policy are
recorded outside the UI. Tests compare the authored-tree oracle with Git numstat,
parentage and reachability and assert the meaningful revert/rename outcomes.

## End-to-end Review workspace

Code and review share one workspace. **Changes only** filters changed files and
objects and shows unified hunks with one scroll surface. Added lines are green
and removed lines red, with base/head line numbers and explicit change markers.
Expand **Review activity and findings** for notes, hotspots, history and exports.
Existing `/review` links redirect into Code with their comparison context.

Open **Code**, select **Compare changes**, and set base to `target` and
review/head to `review/history`, then open the review. The page compares those
exact trees and labels their resolved hashes. It does not automatically select
a merge base or simulate a merge. `WORKTREE` is supported as a tree comparison;
commit activity is explicitly unavailable for that mode.

1. Expand directories and select files, including `unassigned/`. Search paths to
   retain their directory context. Removed files have before source and an absent
   after side; binary files are labeled without fabricated text. Source panes
   contain the two exact revisions, not the live checkout.
2. Find checkout in Branch hotspots: three touches and churn six, with zero net
   changes. Select that row. Three touching commits retain their original
   messages and individual patches; the final net patch is empty. Filter by
   `assets` to inspect the custom commit and binary marker. Filters select paths
   while row statistics retain full-range totals. The counting policy, binary
   uncertainty and included paths are visible.
3. Open an object from Objects and signatures or a changed-symbol link. Inspect
   its signature, observed references and source evidence. Possible transitive
   module impact is reverse dependency reachability, not a runtime-call claim.
   The complete comparison retains violations, requirement and evidence changes.
4. Select checkout, write a review note and mark it Viewed. Advance the same
   review branch with another checkout edit and commit, then refresh. The note
   remains bound to its original hash and context and needs reconsideration.
   Restart the server: the finding and viewed state must survive. An old save
   form is rejected after the comparison changes. Saving again acknowledges the
   new comparison; earlier records remain in `.seamgraph/reviews.jsonl`.
5. Export JSON, Markdown and HTML. Each retains the complete normalized
   comparison, source reference evidence, history and latest findings with their
   original anchors. Full source is included only for the selected file; saved findings also retain
   their original bounded source excerpts. HTML
   is self-contained with no scripts or external assets. Display filters are
   recorded; exports keep the complete evidence rather than discarding facts
   hidden by those filters.

The browser acceptance test performs this sequence against a freshly generated
repository, checks history against the independent oracle, commits a real follow-up
change, restarts the actual server and inspects the exported artifacts. It also
checks removed/binary source handling, stale-save rejection and mobile overflow.

Current limits are explicit: the review requires an analyzable configured Git
project; object details show available adapter signatures and reference locations,
not inferred definition positions. Notes attach to whole files (one latest note
and viewed/resolved state per path and selected ref pair), not diff hunks or
symbols. Anchor digests include both compared file contents; unavailable anchors
retain their original context. Activity excludes merges, follows no renames and
is bounded to 500 commits. Patches and source displays are bounded to 2 MiB;
binary or oversized text stays unavailable. The review log is append-only,
serialized between writers, and bounded to 16 MiB; archive a full log before
adding notes. The later roadmap still covers configuration-free symbol trees,
automatic merge-base selection, richer filtering and hotspot visualizations,
hunk/symbol anchors, and more concise review reports.
