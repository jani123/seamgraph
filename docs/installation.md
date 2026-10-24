# Install SeamGraph without building it

The release workflow publishes prebuilt binaries to
https://github.com/jani123/seamgraph/releases. Once the first release containing
this workflow has been published, use:

```sh
curl -fsSL https://github.com/jani123/seamgraph/releases/latest/download/install.sh -o /tmp/install-seamgraph.sh
sh /tmp/install-seamgraph.sh --languages python,typescript
```

The installer supports Linux amd64/arm64 and macOS arm64 (Apple Silicon),
detects the platform, downloads the corresponding binary,
verifies its SHA-256 checksum and installs it in `~/.local/bin`. It does not need
Go, Node or Python to install SeamGraph. It needs the usual curl, tar, awk and
sha256sum (or macOS shasum) commands. Add `~/.local/bin` to PATH if it is not already
there. The installer prints the installed version and PATH guidance.

For the shorter pipeline form:

```sh
curl -fsSL https://github.com/jani123/seamgraph/releases/latest/download/install.sh | sh -s -- --languages python
```

Windows amd64/arm64 ZIPs are also release assets; extract `seamgraph.exe` into a
directory on PATH. The automatic shell installer supports Linux/macOS. Windows
binaries are cross-compiled; the release workflow smoke-tests native startup on a
Windows runner. Full native Windows adapter behavior is not certified by that
startup check. WSL uses the Linux installer.

## Install into any project with Devbox

After a GitHub release is published and its hashes are committed in
`nix/release.json`, add SeamGraph to an existing project's Devbox environment.
Before the first release is pinned, the flake fails with an explicit setup message:

```sh
cd your-project
devbox init # only for projects without devbox.json
devbox add 'github:jani123/seamgraph#seamgraph'
devbox run seamgraph init --root .
# Edit seamgraph.yaml to define your modules and seams.
devbox run seamgraph serve --root .
```

Commit `devbox.json`, `devbox.lock` and `seamgraph.yaml`. The lock pins the
packaging revision; updating is explicit with `devbox update`. For an explicit
packaging pin, use `github:jani123/seamgraph/<packaging-commit>#seamgraph`,
replacing the placeholder with a commit containing the desired release metadata.
The binary version is pinned separately in that commit's `nix/release.json`.
An application release tag is created before the packaging update, so using
that tag as the flake reference can select the previous release or no release.

The flake supports Linux on Intel and ARM, and macOS on Apple Silicon; Windows users need WSL.
It downloads the matching prebuilt archive from GitHub Releases, verifies the
SHA-256 hash committed in `nix/release.json`, and installs the binary without
modifying it. No Go compiler, npm build, frontend generation, Cachix account,
or custom Nix cache configuration is required. Language runtimes still come
from the consuming project. The flake never follows `latest` at installation
time; unavailable or modified release assets cause installation to fail.

Reuse the consuming project's language runtimes. Add only missing dependencies,
for example:

```sh
devbox add python@3.12 # Python projects
# devbox add nodejs@22 # TypeScript projects
# Go projects: provide Go 1.26.10 or later, matching the project's requirements.
devbox run seamgraph doctor --languages python
```

Git and OpenSSH are needed for remote repository access. Project dependencies
and test tools remain part of that project's setup. Installation never creates
or changes `seamgraph.yaml`; `init` is a separate, explicit command.

Remove the package with `devbox rm` using its package reference. Configuration
and analysis outputs remain in the project.

To test local packaging from a Git checkout with tracked flake files, run
`sh scripts/test-devbox.sh`. This creates an isolated
consumer environment, installs the pinned release plus Python, and verifies
runtime discovery, a real scan, and policy checking. To test candidate archives
before publication, generate their metadata and run:

```sh
python3 scripts/update-devbox-release.py v2026.10.1 --assets-dir dist --output dist/devbox-release.json
sh scripts/test-devbox-release.sh dist
```

The candidate test seeds Nix's fixed-hash downloads from local archives and uses
a temporary copy of the same flake; it does not change the committed release pin.
The Devbox workflow runs it on all three supported platforms, including a native
Apple Silicon macOS runner. Release publication waits for these checks.

## Automatic Devbox package updates

After the checked source is released, the workflow downloads `SHA256SUMS` and
all three Linux/macOS archives from that exact published version. Only after
verification does it commit `nix/release.json` to `main`. The commit uses the
GitHub Actions token, so it does not trigger another workflow. Consumers can run
`devbox update` to adopt it.

Until the first automatic release finishes, `nix/release.json` is `null` and the
flake reports that no release is pinned. Passing the earlier CI jobs alone does
not make public installation available: the release and pin-update jobs must
also succeed. No placeholder hashes or source-build fallbacks are used.

For manual recovery or forks, the updater remains available:

```sh
python3 scripts/update-devbox-release.py v2026.10.1
sh scripts/test-devbox.sh
```

Use an actual published version; for a fork, pass `--repository owner/repo`.
A failed download, missing checksum, or mismatch leaves the existing pin intact.
Review, commit, and push a manual metadata update. Do not move or overwrite the
application release tag to include this later update. The hashes provide
integrity against the reviewed pin; the updater trusts the selected GitHub
release and its manifest, rather than independent code signing.

## Select the dependencies you actually need

| Selection | External runtime | Already shipped inside SeamGraph |
| --- | --- | --- |
| `python` | Python 3.10+ | Standard-library AST helper |
| `typescript` | Node 18+ | TypeScript 5.9.3 compiler and standard declarations |
| `go` | Go 1.26.10+ | Go adapter; external Go toolchain remains necessary |
| `git` | Git | Repository workspace management |
| `ssh` | OpenSSH client | Uses your existing agent and known hosts |

`--languages` checks those tools after installing the binary. It does not run
sudo, install system packages or change language versions. Missing runtimes cause
exit code 2 while leaving the binary installed, with guidance to provision tools
in the devcontainer and rerun doctor. The default installer installs the binary
without checking every language runtime.

```sh
seamgraph doctor --languages python
seamgraph doctor --languages typescript,git,ssh
seamgraph doctor --root /workspaces/project
```

Without an explicit selection, doctor uses languages from the project's
`seamgraph.yaml` when one exists. Without a config it checks all supported tools.
Add explicit `git,ssh` checks when preparing remote access. Installing a prebuilt
binary never requires Go just because SeamGraph itself was written in Go.

## Devcontainer setup

Reuse the runtimes your project already provides. For missing runtimes, add only
the matching official Dev Container Features. For a Python/TypeScript project:

```jsonc
{
  "features": {
    "ghcr.io/devcontainers/features/python:1": {
      "version": "3.12",
      "installTools": false
    },
    "ghcr.io/devcontainers/features/node:2": {
      "version": "22"
    }
  },
  "postCreateCommand": "sh .devcontainer/install-seamgraph.sh",
  "remoteEnv": {
    "PATH": "${containerEnv:HOME}/.local/bin:${containerEnv:PATH}"
  },
  "forwardPorts": [8765]
}
```

The official [Python feature](https://github.com/devcontainers/features/tree/main/src/python)
and [Node feature](https://github.com/devcontainers/features/tree/main/src/node)
provide the language runtimes. For Go projects use the official
[Go feature](https://github.com/devcontainers/features/tree/main/src/go) with the
project's required version. Features are independent of SeamGraph's installer.

Merge these fields with an existing devcontainer configuration rather than
replacing its setup. Commit `.devcontainer/install-seamgraph.sh` with a pinned
release, for example after publishing v2026.10.1:

```sh
#!/bin/sh
set -eu
curl -fsSL https://github.com/jani123/seamgraph/releases/download/v2026.10.1/install.sh -o /tmp/install-seamgraph.sh
sh /tmp/install-seamgraph.sh --languages python,typescript
```

Pinning keeps container rebuilds on an intentional SeamGraph version. The
release installer is stamped with its repository and version, so it downloads
matching binaries even if a newer release appears meanwhile. Project dependency
installation (Go modules, node_modules, generated files, test frameworks) remains
part of the project's own setup.

## Versions, upgrades and uninstall

```sh
sh /tmp/install-seamgraph.sh --version v2026.10.1 --bin-dir "$HOME/.local/bin"
seamgraph version
```

Download the latest installer again to upgrade. The binary is replaced only after
a successful download/checksum verification; a failed verification leaves an
existing installation intact. SHA-256 verifies the release archive against the
manifest delivered by the same GitHub release; this is not a separate code-signing
or provenance-attestation system. Remove `~/.local/bin/seamgraph` to uninstall;
managed repositories and caches are retained.

## Coding agent setup

Installing the executable makes it available to agents; project instructions
explain when to use it. From the consuming project, run:

```sh
devbox run seamgraph agent setup
```

For a native installation, use `seamgraph agent setup`. Use `--root PATH` to
select another project. Setup works before `seamgraph.yaml` exists and does not
create or modify architecture configuration.

Setup installs two files:

- `AGENTS.md`: a short section describing SeamGraph, task triggers, useful
  commands, and architecture verification expectations. Existing project
  instructions outside the `<!-- seamgraph:begin -->` and
  `<!-- seamgraph:end -->` markers are preserved byte for byte.
- `.agents/skills/seamgraph/SKILL.md`: a reusable skill covering exploration,
  stable IDs, source evidence, violations, revision comparison, diagnostics,
  and contract verification. The AGENTS.md section links to this file so agents
  can read it even without automatic skill discovery. Automatic discovery
  depends on the agent's supported skill locations.

Commit both files with the project. The generated commands use
`devbox run seamgraph` when `devbox.json` or `devbox.jsonc` exists in the selected
root, otherwise `seamgraph`.

Rerun setup after updating the binary. It replaces the managed section and
managed skill with the guidance bundled in that binary; unchanged files are
not rewritten. Keep project-specific additions outside the AGENTS.md markers,
since edits inside the section or managed skill are replaced on update.

Setup validates both destinations before writing. Malformed, reversed, or
duplicate AGENTS.md markers and existing skills without SeamGraph's ownership
marker produce an error without changing either file. Resolve the markers or
move the custom skill before retrying. Symlink destinations and paths escaping
the selected project are rejected. Each changed file is replaced atomically;
an I/O failure can leave one file updated, and rerunning setup completes the
remaining update.

## Publishing a release

Push source changes to `main`. The **checks** workflow runs the acceptance suite
and container checks first, then calls the reusable release workflow. Pull
requests and other branches run the Devbox candidate tests after those checks
and never publish releases. The **checks** workflow also supports manual dispatch
on `main` for bootstrapping or retrying the pipeline.

Versions use `vYYYY.M.N` in UTC, with no leading zeroes: `v2026.10.1`,
`v2026.10.2`, then `v2026.11.1` when November begins. The sequence is selected
from existing calendar tags. Legacy semantic tags are ignored by this counter.
Release workflows are serialized to prevent two runs allocating the same tag.
Out-of-order runs for already superseded source do not publish an older revision.

The release workflow runs the required Go quality checks and source tests,
cross-compiles five binaries, creates archives/checksums/installer metadata,
and tests native startup on Linux, Apple Silicon macOS, and Windows. The Devbox
installation and scan tests run on both Linux architectures and Apple Silicon
macOS using those exact candidate archives. Only then does the workflow create
the tag and GitHub release, verify the public downloads, and publish the pin.
Tags are created inside the called workflow; the pipeline does not depend on
a bot-created tag triggering a new workflow or require a personal access token.

The publishing job needs `contents: write` and permission to push tags and a
pin commit to `main`. Repository rules must allow those bot writes. If branch
protection blocks the pin commit, the job fails visibly; the published release
remains available and the existing Devbox pin remains unchanged.

Rerunning the original workflow reuses the same version when that source is
already tagged. Published assets are never replaced. Pin publication retries
concurrent branch updates, preserves newer source commits, and refuses to move
the pin to an older calendar version. Pin-only commits do not allocate a release.
If public downloads or the pin push fail after publication, resolve the failure
and rerun the original workflow to finish the pin update.

For local packaging without publishing:

```sh
scripts/package-release.sh v2026.10.1 jani123/seamgraph .build/release
python3 scripts/update-devbox-release.py v2026.10.1 --assets-dir .build/release --output .build/release/devbox-release.json
sh scripts/test-devbox-release.sh .build/release
```

Non-Linux binaries cannot be executed in this workspace; native startup and
Devbox execution are validated by the platform matrix in CI. Creating this
workflow locally does not publish a release: push the commit and wait for the
entire pipeline, including the final pin update, to finish.
