# SeamGraph

SeamGraph is a local Go CLI and embedded browser explorer for architecture in Go,
TypeScript and Python repositories. Define modules and public seams, inspect real
consumer references, enforce boundaries, compare Git revisions, and inspect
explicit contract-test evidence. All adapters normalize to the same versioned IR;
policy, comparison, verification and UI have no language-specific behavior.

## Install a prebuilt release

Successful checks on `main` automatically publish calendar-versioned releases
and update the Devbox pin. After the first automatic release finishes:

```sh
curl -fsSL https://github.com/jani123/seamgraph/releases/latest/download/install.sh | sh -s -- --languages python,typescript
```

No Go installation is needed to install the binary. Choose only the languages you
analyze; the installer checks their existing runtimes. See [installation and
devcontainer setup](docs/installation.md) for pinned versions, runtime provisioning
and Windows downloads.

## Add to a project with Devbox

After the first GitHub release is published and pinned in `nix/release.json`,
run these commands from the project you want to analyze. Devbox must already
be installed. Until then, the flake reports that no release is pinned.

```sh
cd /path/to/your-project
devbox init # only if this project does not already have devbox.json
devbox add 'github:jani123/seamgraph#seamgraph'
```

Reuse your project's existing language runtimes, or add only the ones you need:

```sh
devbox add python@3.12 # for Python analysis
# devbox add nodejs@22 # for TypeScript analysis
# For Go analysis, provide Go 1.26.10 or later as required by your project.
devbox run seamgraph doctor --languages python
```

Initialize the architecture configuration, edit `seamgraph.yaml` to define your
modules and seams, then check the project or open the explorer:

```sh
devbox run seamgraph init --root . # only when seamgraph.yaml does not exist
# Edit seamgraph.yaml before running analysis.
devbox run seamgraph check --root .
devbox run seamgraph serve --root .
```

To teach coding agents when and how to use SeamGraph, run once in the project:

```sh
devbox run seamgraph agent setup
```

This installs a managed `AGENTS.md` section and
`.agents/skills/seamgraph/SKILL.md`. Commit both files. Rerun after upgrading
SeamGraph to update the guidance without duplicating rules. See
[agent setup](docs/installation.md#coding-agent-setup) for update behavior.

Open **http://127.0.0.1:8765**. Commit `devbox.json`, `devbox.lock` and
`seamgraph.yaml` so teammates get the same package revision and configuration.
For intentional upgrades, run `devbox update`. To keep a specific package
definition, replace the unversioned reference with
`github:jani123/seamgraph/<packaging-commit>#seamgraph`, substituting a real
commit containing the desired `nix/release.json`. The application release tag
precedes its packaging update, so that tag may still reference an older binary.

Supports Linux on Intel and ARM, and macOS on Apple Silicon (Windows through WSL). The flake
downloads the matching prebuilt GitHub release archive, verifies its pinned
SHA-256 hash, and installs the binary. Installing SeamGraph requires no Go
compiler, frontend build, or Cachix setup. Project language runtimes, dependencies
and test runners remain part of your project's setup.
See [Devbox installation](docs/installation.md#install-into-any-project-with-devbox)
for more details.

Releases use `vYYYY.M.N`, for example `v2026.10.1`: the first release in
October 2026 (UTC). Each successful source update on `main` is eligible for a
release after the full checks and native package tests pass. The workflow
publishes GitHub assets, verifies their downloads, and commits the Devbox pin
automatically. Pin-only commits do not produce another release.

To start or retry the pipeline manually, run the **checks** workflow on `main`
from GitHub Actions. No manually created tag or personal access token is needed.
See [release automation](docs/installation.md#publishing-a-release) for recovery
and repository permissions.

## Build from source

```sh
go mod download
go build -o .build/seamgraph ./cmd/seamgraph
.build/seamgraph check --root testdata/projects/mixed --json
.build/seamgraph serve --root testdata/projects/mixed
```

Open **http://127.0.0.1:8765**. The mixed fixture contains six modules and three
seams across all three languages. No CDN, cloud service, database or LLM is used.
Node 18+ is required when TypeScript modules are configured; Python 3.10+ is
required for Python. The TypeScript compiler, its standard declaration libraries,
Python helper and frontend assets are embedded in the binary. Scanning downloads
nothing; Go project dependencies must already be available locally.

## Try a realistic local review

Generate the [order fulfillment example](examples/order-fulfillment/README.md)
with Go, TypeScript and Python, local fakes, reproducible review branches and
explicit expected findings:

```sh
python3 scripts/examples/generate.py .build/fulfillment
.build/seamgraph serve --root .build/fulfillment
```

The [scenario guide](examples/order-fulfillment/SCENARIOS.md) covers boundary
violations and fixes, cancellation, incompatible consumers, evidence freshness,
Git history edge cases and a bounded larger graph. `task examples:test` verifies
the supported workflows independently of the browser.

Open **Review** to browse the comparison's directories, inspect adapter objects,
drill from hotspots into commits and patches, save file notes and viewed state,
and export the complete evidence as JSON, Markdown or standalone HTML. Findings
retain their original revision and context across commits and server restarts;
changed anchors are flagged for reconsideration. See the
[review workflow and limits](examples/order-fulfillment/SCENARIOS.md#end-to-end-review-workspace).

## Run with bundled analysis runtimes

```sh
SEAMGRAPH_REPOSITORY=https://github.com/pallets/itsdangerous.git docker compose up --build
```

Open http://localhost:8765. The runtime image contains Go, Node, Python and Git/SSH;
repositories and dependency caches persist in a volume. See [deployment](docs/deployment.md)
for SSH agent forwarding, local projects and dependency preparation. For native
installations, `seamgraph doctor` checks the external tools.

## Open a remote repository

```sh
seamgraph open https://github.com/pallets/itsdangerous.git
seamgraph open git@github.com:your-team/project.git --ref main
```

Run `go install ./cmd/seamgraph` to install the CLI locally. Remote opening uses
Git and your existing SSH/HTTPS credentials, creates a persistent workspace, and
starts the explorer. Repositories without `seamgraph.yaml` receive an exploratory
config with directory modules and no inferred seams. See [repository setup and
deployment](docs/repositories.md) for config overrides, dependencies and limits.

## Using your own project

Run `seamgraph init --root /path/to/project`, edit the placeholder configuration,
then scan or check. Module roots are explicit, disjoint directories; each root
owns its subtree. [Configuration](docs/configuration.md) covers symbol selectors,
entrypoints, Python import roots, TypeScript configuration and rule precedence.

```sh
seamgraph validate --root /path/to/project
seamgraph scan --root /path/to/project --format json
seamgraph check --root /path/to/project --format sarif
seamgraph snapshot --root /path/to/project --output architecture.json
seamgraph inspect seam:world.reader --root /path/to/project --tests --json
seamgraph diff --root /path/to/project --base main --head HEAD --json
seamgraph diff --root /path/to/project --base HEAD --head WORKTREE --json
```

Exit codes: **0** successful/clean, **1** architecture or enforced contract
violations, **2** invalid configuration, incomplete strict analysis or runtime
errors. `scan` reports facts; `check` enforces policy. `validate` checks declarations
only. Relaxed uncertainty emits visible warnings. A module import is never proof
of interface use. Structural interface compatibility is shown as a type fact,
not as an observed consumer dependency.

Browser details include copyable IDs and agent prompts, contract members,
consumers, evidence and violations. Expand a module's seams without exposing
implementation symbols in the overview. Rule proposals preview their YAML changes;
only explicit confirmation writes configuration, with a backup and concurrent
change check. Source viewing is restricted to evidence inside the project.

The explorer's **Compare changes** control defaults to **Working changes**:
HEAD versus the final checkout contents, including staged, unstaged and untracked
nonignored files. **Revisions** compares two committed refs. Both sides must have
a valid `seamgraph.yaml`. Comparisons materialize temporary trees and never alter
the checkout or index.

Module badges distinguish source edits (`~ Modified`), seam changes and configuration
changes. New or resolved violations add a separate `Policy changed` badge. Green marks additions; red dashed shapes mark removals; amber dotted
arrows mark modified connections. A connection remains modified when some of its
references are removed but others still connect the same modules. Violations keep
a separate red border or halo. Select a module or arrow for changed files,
contract members and before/after references, including removed objects. Use
**Changed + neighbors** to focus the graph or **Clear comparison** to reload the
current architecture.

File attribution follows each side's module roots. Exact-content moves are shown
as renames; edited moves appear as a deletion and addition. Root configuration
changes are reported separately from module source changes. These indicators
report changed inputs and architecture, rather than estimating change complexity.

## Contract verification

Declare behavioral requirements and reviewed test mappings in
`seamgraph-contracts.yaml`. Then run supported tests or import a report with its
explicit run metadata:

```sh
seamgraph test --root /path/to/project --runner go
seamgraph check --root /path/to/project --contracts --json
```

Runners include Go, Vitest and pytest; importers also support Jest JSON and JUnit.
Evidence is tied to the exact Git commit and a conservative project fingerprint.
Implementation, signature, test or requirement changes make old evidence stale.
Missing requirements/evidence remain unverified or unknown. Member execution
coverage and declared requirement verification are separate metrics. Passing tests,
static links and line coverage alone never imply complete behavioral verification.
See [the contract evidence guide](docs/contracts.md).

## Development

Use Go 1.26.10, Node 22 and Python 3.12, or the pinned Devbox setup:

```sh
devbox install
devbox shell
task doctor
task deps           # install pinned development dependencies once
task check          # Go, helper and browser checks
```

Generated Go, bundled compiler/runtime libraries and frontend assets are checked
in, so building the binary needs only Go dependencies. `task generate` rebuilds
assets from pinned npm packages and Templ components. Browser tests use an
npm-packaged Chromium so no separate browser download is needed. Linux needs
NSPR/NSS/Expat; the Nix test launcher uses installed Devbox libraries and patchelf.
See [container instructions](.devcontainer/README.md) for the restricted-network
workspace. The server binds to loopback only.

[Progress](docs/agent-progress.md), [IR semantics](docs/ir.md),
[adapter limits](docs/adapters.md), [policy and comparison](docs/policy-and-diff.md),
and [architecture decisions](docs/adr/0003-adapters.md) describe the implementation
and its limits. The full specification is [SEAMGRAPH_PLAN.md](SEAMGRAPH_PLAN.md).

## SeamGraph's own architecture

The root [seamgraph.yaml](seamgraph.yaml) defines explicit modules, public seams and
default-deny connections for SeamGraph itself. Run `task architecture:check`, or
`.build/seamgraph serve --root .` to explore it. CI and releases enforce the same
configuration. See [self-architecture](docs/self-architecture.md) for boundaries
and analysis scope.

## Live development

From the host, run `task devcontainer:rebuild` once after updating the container,
which starts Air automatically. `task devcontainer:dev` follows its logs and
prints Docker's randomly assigned localhost URL. `task devcontainer:url`
prints it again. This works with the standalone Dev Container CLI; VS Code is
optional. See [development setup](docs/development.md).
