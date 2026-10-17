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
