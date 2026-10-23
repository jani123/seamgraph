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
python3 scripts/update-devbox-release.py v0.1.0 --assets-dir dist --output dist/devbox-release.json
sh scripts/test-devbox-release.sh dist
```

The candidate test seeds Nix's fixed-hash downloads from local archives and uses
a temporary copy of the same flake; it does not change the committed release pin.
The Devbox workflow runs it on all three supported platforms, including a native
Apple Silicon macOS runner. Release publication waits for these checks.

## Update the Devbox package after a release

The first release has not been published or pinned by this implementation.
`nix/release.json` is deliberately `null` until real release assets are available;
there are no placeholder hashes or silent source-build fallbacks.

The release workflow publishes the archives, `SHA256SUMS`, and a verified
`devbox-release.json`. After publication, run:

```sh
python3 scripts/update-devbox-release.py v0.1.0
sh scripts/test-devbox.sh
```

Replace the example with the actual published version. The updater downloads the
checksum manifest and all three Linux/macOS archives from that exact release,
verifies every archive, and only then atomically replaces `nix/release.json`.
A failed download, missing checksum, or mismatch leaves the existing pin intact.
For a fork, pass `--repository owner/repo`.

Ensure the flake files are tracked before testing a new checkout. Review and
commit the metadata update to the default branch, then publish that packaging
commit. Consumers can run `devbox update` to adopt it. Do not move or overwrite
the application release tag to include this later update. The hashes provide
integrity against the reviewed pin; the updater trusts the selected GitHub
release and its checksum manifest, rather than independent code signing.

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
release, for example after publishing v0.1.0:

```sh
#!/bin/sh
set -eu
curl -fsSL https://github.com/jani123/seamgraph/releases/download/v0.1.0/install.sh -o /tmp/install-seamgraph.sh
sh /tmp/install-seamgraph.sh --languages python,typescript
```

Pinning keeps container rebuilds on an intentional SeamGraph version. The
release installer is stamped with its repository and version, so it downloads
matching binaries even if a newer release appears meanwhile. Project dependency
installation (Go modules, node_modules, generated files, test frameworks) remains
part of the project's own setup.

## Versions, upgrades and uninstall

```sh
sh /tmp/install-seamgraph.sh --version v0.1.0 --bin-dir "$HOME/.local/bin"
seamgraph version
```

Download the latest installer again to upgrade. The binary is replaced only after
a successful download/checksum verification; a failed verification leaves an
existing installation intact. SHA-256 verifies the release archive against the
manifest delivered by the same GitHub release; this is not a separate code-signing
or provenance-attestation system. Remove `~/.local/bin/seamgraph` to uninstall;
managed repositories and caches are retained.

## Publishing a release

Commit the CLI source, installer, packaging helper and workflow before tagging.
The corrected `.gitignore` now ignores only the root `/seamgraph` executable,
allowing `cmd/seamgraph/` to be included in Git. It was previously hidden by an
unanchored ignore rule and must be included in the commit.

Push a version tag on the desired committed source, for example `v0.1.0`. The
release workflow runs Go and installer tests, cross-compiles five binaries,
packages tar.gz/ZIP assets, creates SHA256SUMS and stamps install.sh. Native
Linux/macOS/Windows startup smoke tests run before publication. Tags containing
a hyphen publish a prerelease. Published releases are not overwritten on reruns.
GitHub Releases are [based on Git tags](https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases).

Local packaging:

```sh
scripts/package-release.sh v0.1.0 jani123/seamgraph .build/release
```

The local verification built all supported targets and exercised the actual installer
with controlled downloads, including tampered checksums, existing installations,
missing runtimes and version-pinned releases. Non-Linux binaries cannot be
executed in this workspace; native startup is delegated to release CI. The
workflow and assets have not been published by the coding session.
