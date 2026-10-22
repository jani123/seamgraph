# Agent progress — 2026-10-16

## Current outcome

A working local CLI and embedded browser now support Go, TypeScript, Python and
mixed projects through one language-neutral IR. Implemented the v1 feature path
through CLI policy, contract evidence, browser exploration, exact committed Git
comparison, confirmed rule editing and copyable IDs. The supported paths pass
local acceptance checks. Explicit limits below remain; this is not a claim of
complete semantic proof for arbitrary programs or of complete behavioral coverage.

The user approved continuing through phases without stopping for phase approval,
including all three adapters. Literal, disjoint roots were confirmed. `paths: [.]`
represents an entire project and cannot overlap another module. No production
behavioral requirements or reviewed test mappings were invented. Acceptance-test
requirements describe only their authored synthetic fixtures.

No prototype/application source existed at kickoff; only development tooling and
the specification were provided. There is still no Git repository for this
workspace, so phase commits could not be made. Git was not initialized here;
Git acceptance tests create and clean up disposable repositories.

## Phase results

- **P0/P1:** strict real YAML parsing, duplicate/unknown/reference validation,
  explicit ownership, versioned IR, stable IDs, canonical JSON/goldens, synthetic
  language-equivalence tests and import-boundary tests. Initial prototype
  limitations were audited as absent, not attributed to nonexistent code.
- **Pre-adapter gate:** shared normalized policy and comparison cores were moved
  ahead of adapters with user approval. Identical policy/comparison suites run
  against all three synthetic language-origin fixtures.
- **P2 Go:** compiler package/type/AST resolution, aliases, interfaces, structural
  satisfaction, named public methods/fields, symbol and deep-package bypasses,
  current build tags/workspace context, source evidence and load/cycle errors.
  Structural conformance never creates an observed dependency by itself. Runtime
  downloads and module edits are disabled during analysis.
- **P3 TypeScript:** bundled Compiler API, tsconfig paths, barrels/re-exports,
  type-only imports, named/default exports, public class/interface members and
  structural satisfaction, deep-import evidence and dynamic diagnostics. Compiler,
  standard declarations and licenses are embedded; no runtime npm installation.
- **P4 Python:** AST/import resolver, src/import roots, namespaces, relative imports,
  aliases, static re-exports, literal __all__, star and namespace re-exports, public
  class methods, deep imports, and explicit dynamic/unresolved diagnostics.
- **P5 CLI:** init/validate/scan/check/snapshot/inspect/diff/serve; text/JSON/SARIF
  architecture reports; exact stable violation baselines; 0/1/2 exit semantics.
  Baselines cannot hide incomplete analysis. Errors never fabricate clean results.
- **P5b contracts:** strict authored requirements/mappings, reviewed assertion
  linkage, actual Go/Vitest/pytest execution, Go/Vitest/Jest/JUnit imports, exact
  revision/input-context freshness, normalized statuses and separate metrics.
  `inspect --tests`, `check --contracts --json`, fingerprint and import-tests work
  offline. Artifact imports require producer metadata and exact artifact digest.
  Stores use exclusive writer locks, fsync/temp/rename, and idempotent imports.
- **P6 UI:** Templ/HTMX/Tailwind/Alpine/Cytoscape with bundled local assets; module
  overview, seam expansion/details, consumers, evidence, search/violation filters,
  clear contract statuses and qualified metrics. Real browser tests include 100
  modules/500 edges. Source viewing is root-constrained and evidence-restricted.
- **P7 Git:** verified exact commit trees via ls-tree/cat-file into temporary
  directories, with no checkout/hooks/working-tree changes. Export-ignore and
  export-subst cannot alter analysis inputs. CLI/browser before/after records,
  signature changes, consumers, policy regressions, requirements and verification
  changes; session caches by commit/config/context; removed-ID revision inspection.
- **P8 editing/hardening:** exact targeted YAML preview; cancel invalidates proposal;
  explicit confirmation, validation, atomic replacement, backup and concurrent-edit
  rejection. Loopback address/host enforcement, origin/token checks and local CSP.
- **P9:** stable module/seam/symbol/edge/violation identities; Copy ID and Copy agent
  prompt; CLI resolves copied browser IDs for all three languages; unknown IDs fail
  with candidates; old identities can be inspected against an explicit revision.

## Validation

Passed locally on the implemented feature paths:

- Offline `task check`: build, gofmt check, vet, Go suites, 5 TypeScript helper tests,
  3 Python helper tests, 3 actual-runner acceptance tests and 9 Chromium browser tests.
- Offline `go test -race ./...` (rerun for final source changes).
- Asset regeneration compared by SHA256 to ensure embedded/generated bytes are
  reproducible. Generated helpers, standard libraries, templates and assets are
  checked in; `task generate` uses pinned npm dependencies and Templ.
- Real Go/Vitest/pytest runs each demonstrate reviewed requirement verification,
  implementation-change staleness, a successful rerun, and failed assertions
  producing failed status/nonzero enforcement. No member coverage is fabricated.
- Browser: selection, seam expansion, clipboard actions, IDs resolved through CLI,
  module consumers, local-only requests, preview/cancel/confirm, invalid proposals,
  concurrent changes, graph scale, Git overlays and removed revision identities.
- Git extraction tests preserve uncommitted files and exact committed bytes even
  with export attributes; cache tests include changed analysis context.
- Mixed-language strict architecture check succeeds with six modules/three seams.
  Its configured Python-to-Go grant does not create an invented observed edge.
  Direct `check --contracts --json` returns exit 0 in report mode with
  `seam:go-world.reader`, `seam:py-world.reader` and `seam:ts-world.reader` all
  **unverified**: the demo intentionally has no declared behavioral requirements
  or execution evidence. The actual-runner acceptance fixtures verify their
  explicitly authored requirements; this does not certify the demo's behavior.

The environment blocked Playwright CDN browser downloads. Tests instead use a
pinned npm-distributed Chromium, with installed NSPR/NSS/Expat and a Nix-aware
launcher. No security/CSP checks were disabled in the application. Remote CI was
configured but has not executed on a remote service.

## Files and architecture

- `cmd/seamgraph/`: CLI orchestration, evidence persistence and end-to-end tests.
- `internal/ir/`, `config/`, `policy/`, `snapshot/`, `verification/`, `revision/`:
  normalized architecture, strict policy/config, comparison and evidence freshness.
- `internal/analyzer/`: common boundary, Go/TS/Python adapters, registry and runner
  report/command adapters. Language-specific logic stays inside this boundary.
- `internal/gitdiff/`: exact isolated revisions and bounded session cache.
- `internal/server/`, `ui/`: secured local endpoints, generated Templ components,
  bundled assets, source evidence and confirmed editing.
- `testdata/ir/`, `testdata/projects/`: shared synthetic models, individual-language
  projects and a functioning mixed project.
- `tools/ts-analyzer/`, `py-analyzer/`, `ui/`, `test-runners/`: helper sources,
  reproducible bundles, pinned tests and actual runner/browser acceptance suites.
- `README.md`, `docs/`, Taskfile, Go/npm/Python dependency manifests, Devbox manifests,
  .gitignore and CI: current usage, architecture decisions and development workflow.

Core/rules/comparison/web/verification consume normalized data and cannot import
language adapters/parsers. Module consumers are derived from observed import/use/
call relations; declaration exposure and structural conformance stay distinct.
Multiple public/deep access paths preserve precise bypass evidence when merged.

## Explicit limits and follow-up work

- Local imports into project directories without a declared module produce explicit
  uncertainty diagnostics; strict policy fails and relaxed policy warns.
- Export-binding-only renames can be missed when they retain the same resolved
  declaration identity; full public alias identity tracking remains follow-up work.
- TypeScript currently requires one shared tsconfig per scan; independent project
  reference builds are rejected explicitly. JS-only projects are not supported.
- Python scoped/conditional imports, legacy string annotations, custom import hooks,
  nonliteral/mutating exports and dynamic attribute/import behavior produce explicit
  unknown/dynamic diagnostics where detected. Full flow-sensitive Python analysis
  and runtime data-flow proofs are not claimed. Strict uncertainty fails; relaxed
  policy reports warnings without inventing facts.
- Raw coverage-file importers are optional follow-up work. The normalized model
  supports distinct coverage evidence; built-in CLI collectors import executions.
  Without current measured evidence, member execution coverage is explicitly unknown.
  Passing reviewed requirements never imply proof of undeclared behavior.
- Cross-language declarations express policy intent. General RPC/FFI/runtime bridge
  discovery requires a mechanism-specific evidence adapter and is not inferred.
- Source roots remain literal/disjoint. Source symlinks are rejected. Git tree
  symlinks/submodules are rejected; revision operations currently require config at
  the Git repository root. No implicit identity migrations or rename inference.
- Browser safe editing requires block-style policies. It preserves unrelated bytes
  and serializes only the selected rule list. Actual source code is not edited.
- Project runtimes and ordinary dependencies must already be installed for offline
  analysis/testing. Freshness is intentionally conservative and can invalidate runs
  for unrelated project inputs outside excluded artifact/cache directories.

See docs/adapters.md and docs/contracts.md for supported semantics and limits.

Graph loading failures now display the analysis or missing-renderer error inside the graph panel. Browser regression tests cover both failure paths.

The explorer uses the full viewport with a flexible graph, independently scrolling details, a mobile stacked layout and graph resizing on container changes. Browser checks cover desktop, tablet and phone dimensions.

## Remote repository workspaces

`seamgraph open <git-url>` now supports public HTTPS and SSH repositories, branch/
tag/commit selection, persistent isolated workspaces, local config overrides and
prepare-only JSON output. Missing configs receive an explicitly exploratory
ownership draft. Git blobs are materialized without checkout filters/hooks; the
existing symlink/submodule restrictions remain. There is no in-memory VFS,
automatic dependency installation, workspace eviction or hosted multi-tenant
service. See docs/repositories.md for deployment and credential behavior.

Acceptance covers remote branch/tag resolution, raw blob materialization, earlier
edit preservation, failed-fetch cleanup, SSH transport via a local stand-in and
CLI preparation followed by actual analysis. A real public HTTPS fetch and scan
of pallets/itsdangerous succeeded at commit
672971d66a2ef9f85151e53283113f33d642dabd.

## Packaged analysis runtime

A production-oriented local Dockerfile/Compose setup now packages Go, Node,
Python, Git/SSH, certificates and basic C tooling. It uses persistent workspaces
and caches, optional SSH-agent forwarding, an unprivileged runtime user and a
localhost-published UI. `--allow-network` explicitly enables container binding
without relaxing HTTP Host/Origin/CSRF checks. `seamgraph doctor` checks external
runtime availability and minimum versions. Project dependencies and test runners
remain explicit project setup. CI now includes image/runtime/HTTP smoke checks.
Docker/Podman are unavailable in this workspace; the image build has not run
locally. See docs/deployment.md for exact usage and remaining platform limits.

## Downloaded installers and release packaging

Added a Linux/macOS amd64/arm64 installer for jani123/seamgraph release assets,
SHA-256 verification, atomic replacement, version pinning and selective runtime
checks. The installer reuses project runtimes and does not modify system packages.
Doctor now accepts --languages and detects languages from project configuration.
Tagged release CI builds six Linux/macOS/Windows binaries, runs native startup
smokes and publishes archives/checksums/a stamped installer. All six targets were
built locally; native non-Linux execution remains CI work. No release was pushed
or published by this session.

Fixed the unanchored `seamgraph` .gitignore rule, which had hidden the entire
cmd/seamgraph source directory from Git. The now-visible cmd directory must be
included in the next source commit. See docs/installation.md.

## Self-enforced architecture

Root seamgraph.yaml declares 16 disjoint Go modules and 23 named public seams,
default-deny rules and strict uncertainty. Every allow targets a seam. The actual
scan resolves 455 relations with zero violations/diagnostics. Normal checks and
release CI enforce the root configuration; negative tests exercise prohibited
core/web adapter access, concrete adapter bypass, CLI UI/rule editing, repository
comparison and UI verifier execution. The analysis subtree remains one module;
embedded JavaScript/Python and tests are covered by their separate suites, not
misrepresented as source-language graph edges. See docs/self-architecture.md.

## Air development and dynamic ports

Added pinned Air v1.65.3, task dev:setup/dev:tools/dev, generated-asset rebuilds,
dependency-tree exclusions and per-session dynamic listen addresses. Server
startup now binds before announcing the actual URL and optionally records it
for Air restart reuse. Devcontainer process-based port forwarding replaces the
hard-coded port list and permits a different available local port. A real Air
smoke run rebuilt twice while retaining its OS-assigned port and served the self
graph. Native listener tests cover ephemeral-port announcement/shutdown; IDE host
forwarding remains untestable in this headless workspace. See docs/development.md.

## CLI-first devcontainer access

The standalone Dev Container CLI is now the primary documented development path.
Host tasks devcontainer:dev/url launch Air and discover the published host URL by
Docker labels and actual Compose project identity. The workspace uses a stable
internal backend port; Docker assigns a localhost host port to a fixed-target
Nginx ingress relay on the private/internet networks. Workspace isolated routing,
Squid egress policy and capabilities are retained. Native task dev keeps its
OS-selected per-session loopback port; VS Code support remains optional. Unit
checks cover discovery and launcher modes. CI now exercises the real relay with
an isolated backend and Host validation; Docker execution remains unavailable
in this workspace. Recreate the container to apply the new service/mapping.

## Module architecture refactoring — 2026-10-18

Implemented the self-architecture refactoring in docs/self-architecture.md.
Application workflows now own inspection/revision lifetimes, comparison caching,
evidence sequencing and rule proposal state through consumer-owned capability
ports. CLI keeps dispatch/output; HTTP keeps routing/security. Project storage,
exact Git/workspace operations and fixed test runners implement separate effects.
Pure project definitions no longer load files or construct engine policy inputs.
Shared analysis protocol/builders moved to a disjoint root; bootstrap registers
Go, TypeScript and Python implementations, with each adapter independently owned.

The root configuration now declares 20 modules and 31 capability seams. Web cannot
collect evidence or initialize projects, CLI cannot confirm rules, and UI has no
engine/query/command execution grant. Production-source probes verify actual
forbidden connections and module-level acyclicity; import checks guard pure and
application code against infrastructure access. Domain data references remain
explicit instead of being hidden behind an opaque shared object.

The graph now uses dependency layers, preserves cycles in their own layer,
distinguishes callable/data/import/construction references, focuses module/seam
neighbors and connects expanded seams to consumers. Module labels fit rectangular
nodes. All references and original evidence remain accessible. The live server
serves 20 modules, 31 seams and zero violations; the additional explicit adapter
boundaries intentionally increase the complete dependency graph's pair count.

Validation: task check passes, including ten browser tests, pure graph projection
checks, installer/devcontainer scripts, compiler/AST helpers and real Go/pytest/
Vitest runner acceptance. Full Go race tests passed, followed by race checks of
the final application/storage/server/boundary changes. The latest seam-focus
browser case also passes. A live preview is saved at .build/architecture-after.png.


## Dependency-aware graph ordering — 2026-10-22

Replaced alphabetical row ordering with bounded barycenter sweeps, temporary
vertices for long dependencies, exact adjacent-swap crossing deltas, and weighted
neighbor alignment with enforced label spacing. The best layered crossing score
is retained; module-ID ties, canonical input traversal and contiguous cycle
blocks keep layouts reproducible. Internal layout points never enter the graph
or source evidence. No new dependencies were added.

Seven graph tests cover direction, cycles, long arrows, shared endpoints,
alignment, shuffled/duplicate inputs, independent two-layer intersection counts,
and a 100-module/500-edge graph. The latter takes roughly 40–70 ms locally.
On the saved self-graph, straight-segment intersections decrease from 58 to 17
in the callable view and from 191 to 111 with all references. Those geometric
measurements are distinct from the internal adjacent-layer crossing score.

Validation for the completed ordering change: all seven graph tests and all twelve
browser tests pass. The browser suite verifies stable positions during hover,
filter/refit behavior, mobile/desktop rendering, and existing explorer workflows.
The focus test now checks its module-details request rather than assuming the
mocked seam name also exists in a shared real fixture changed by earlier tests.
The live preview at .build/architecture-after.png shows the new ordering.


## Orthogonal dependency arrows — 2026-10-22

Changed the graph renderer to horizontal/vertical polylines with 90-degree
corners. A deterministic router distributes per-edge attachment points along
module sides, avoids node rectangles with clearance, and uses horizontal lanes
for dependencies that span columns. Cycles and expanded seams can use alternate
sides. Route coordinates are converted to the bundled Cytoscape segment API;
original edge identities, evidence, violation/revision colors, hover and click
handling remain intact. Routes update after filtering, expansion and dragging.

Geometry tests check obstacle clearance, orthogonality, distributed ports,
cycles/seams, determinism and renderer-coordinate conversion. A real browser
case checks rendered paths, clicks a routed arrow and verifies drag updates.
The live preview at .build/architecture-after.png includes the orthogonal routes.

Validation: all nine graph tests and all thirteen browser tests pass. The final
hover/routing cases were rerun after the drag-end and overlap fallback handling
and both pass. The projection/focus browser fixture now mocks module details as
well as graph data, isolating it from earlier real-project editing tests.
