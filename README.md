# SeamGraph

SeamGraph is a local Go CLI and embedded browser explorer for architecture in Go,
TypeScript and Python repositories. Define modules and public seams, inspect real
consumer references, enforce boundaries, compare Git revisions, and inspect
explicit contract-test evidence. All adapters normalize to the same versioned IR;
policy, comparison, verification and UI have no language-specific behavior.

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

Use Go 1.25, Node 22 and Python 3.12, or the pinned Devbox setup:

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
