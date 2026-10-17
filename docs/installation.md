# Install SeamGraph without building it

The release workflow publishes prebuilt binaries to
https://github.com/jani123/seamgraph/releases. Once the first release containing
this workflow has been published, use:

```sh
curl -fsSL https://github.com/jani123/seamgraph/releases/latest/download/install.sh -o /tmp/install-seamgraph.sh
sh /tmp/install-seamgraph.sh --languages python,typescript
```

The Linux/macOS installer detects amd64/arm64, downloads the corresponding binary,
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

## Select the dependencies you actually need

| Selection | External runtime | Already shipped inside SeamGraph |
| --- | --- | --- |
| `python` | Python 3.10+ | Standard-library AST helper |
| `typescript` | Node 18+ | TypeScript 5.9.3 compiler and standard declarations |
| `go` | Go 1.25+ | Go adapter; external Go toolchain remains necessary |
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
release workflow runs Go and installer tests, cross-compiles six binaries,
packages tar.gz/ZIP assets, creates SHA256SUMS and stamps install.sh. Native
Linux/macOS/Windows startup smoke tests run before publication. Tags containing
a hyphen publish a prerelease. Published releases are not overwritten on reruns.
GitHub Releases are [based on Git tags](https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases).

Local packaging:

```sh
scripts/package-release.sh v0.1.0 jani123/seamgraph .build/release
```

The local verification built all six targets and exercised the actual installer
with controlled downloads, including tampered checksums, existing installations,
missing runtimes and version-pinned releases. Non-Linux binaries cannot be
executed in this workspace; native startup is delegated to release CI. The
workflow and assets have not been published by the coding session.
