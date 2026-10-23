# Deploying the complete analysis runtime

Deploy SeamGraph as a local, single-user container. This bundles analysis tools
with the application, keeps downloaded repositories and dependency caches in a
persistent volume, and publishes the explorer on localhost. It is separate from
the development container and does not include Codex, Devbox or editor tooling.

For installation inside an existing devcontainer without Docker-in-Docker, use
the [release installer](installation.md).

## Runtime and project dependencies

| Component | Included in the image | Role |
| --- | --- | --- |
| SeamGraph binary and browser assets | Yes | CLI, policy/diff, local explorer |
| Go 1.26.10 toolchain | Yes | `go/packages` invokes `go list` and compiler tooling |
| Node 22 and embedded TypeScript compiler | Yes | Compiler-based TypeScript analysis; no global `tsc` installation |
| Python 3.11 (Debian Bookworm) | Yes | Standard-library AST analysis; no pip package needed for scanning |
| Git, OpenSSH client, CA certificates | Yes | HTTPS/SSH fetches and Git revisions |
| C/C++ build tools and pkg-config | Yes | Basic support for Go/cgo projects |
| Target project's dependencies and native libraries | Project-specific | Resolve imports, types and build context |
| pytest, Vitest and other test frameworks | Project-specific | Required only when explicitly running those tests |

Run `seamgraph doctor` or `seamgraph doctor --json` in a native installation. It
checks configured languages (or all tools without a project config), availability and minimum versions, returning exit code 2 if a tool
is missing or too old. The image build runs the same check. Python analysis uses
AST parsing rather than importing project code. Installing a Python project's
runtime dependencies is generally needed for its tests, not for AST parsing.

## Build and start

From the repository root, with Docker and Docker Compose installed:

```sh
SEAMGRAPH_REPOSITORY=https://github.com/pallets/itsdangerous.git \
  docker compose up --build
```

Open http://localhost:8765. To select a revision:

```sh
SEAMGRAPH_REPOSITORY=https://github.com/owner/project.git \
SEAMGRAPH_REF=main docker compose up --build
```

`docker compose down` stops the app and retains data. The `seamgraph-data` named
volume stores repositories and Go/npm/pip caches under `/data`. Recreating the
container opens a new workspace; earlier workspaces and configuration edits stay
in the volume. The terminal prints the exact workspace path and commit.

The runtime image runs as an ordinary user, UID/GID 1000 by default. On Linux,
set `SEAMGRAPH_UID` and `SEAMGRAPH_GID` to your user's numeric IDs before the first
build and data-volume creation if you need matching bind-mount or agent access:

```sh
SEAMGRAPH_UID=$(id -u) SEAMGRAPH_GID=$(id -g) \
SEAMGRAPH_REPOSITORY=https://github.com/owner/project.git \
  docker compose up --build
```

To build and inspect the image without starting a repository:

```sh
docker build --pull -t seamgraph:local .
docker run --rm seamgraph:local doctor
```

The Dockerfile uses versioned Go 1.26.10 and Node 22 Debian image lines; build with
`--pull` to obtain updates. `GO_IMAGE` and `NODE_IMAGE` build arguments can pin
approved image digests for a release. OS packages are resolved at build time, so
a rebuild is not claimed to be byte-for-byte identical. Publish and deploy the
resulting immutable image digest after validating a release.

## Private repositories over SSH

On Linux, use your existing SSH agent and established known-host trust:

```sh
SEAMGRAPH_REPOSITORY=git@github.com:your-team/project.git \
  docker compose -f compose.yaml -f compose.ssh.yaml up --build
```

The overlay forwards `SSH_AUTH_SOCK` and mounts only `known_hosts` read-only; it
does not mount private key files. The container user must be permitted to access
the agent socket. Override `SEAMGRAPH_KNOWN_HOSTS` if the file is elsewhere. Use a
full `ssh://git@host:port/path` URL when a custom host alias/port would otherwise
require SSH config. The overlay does not mount your SSH config automatically.

Docker Desktop agent forwarding uses platform-specific socket paths; use its
supported agent socket, or use native SeamGraph with your workstation's tools.
The bundled client still needs an accessible agent and a trusted host entry.
Private HTTPS repositories likewise need credentials available inside the
container; the host's credential helpers are not automatically available there.

## Local repositories and setup before analysis

A local project can be mounted into the image:

```sh
docker run --rm --init -p 127.0.0.1:8765:8765 \
  -v seamgraph-data:/data \
  -v /absolute/project/path:/project \
  seamgraph:local serve --root /project --addr 0.0.0.0:8765 --allow-network
```

The project mount is writable to support explicit rule edits and test evidence.
Add `:ro` for browsing/checking without those write operations. Bind-mount file
permissions must allow the container user to read the sources.

For remote projects requiring dependency preparation:

```sh
docker run --rm -v seamgraph-data:/data seamgraph:local \
  open https://github.com/owner/project.git --prepare-only
# Use the workspace path printed by that command:
docker run --rm -it -v seamgraph-data:/data --entrypoint sh seamgraph:local
# Inside the container, cd to the workspace and prepare the project as needed.
# For example: go mod download, or the project's documented npm install command.
exit
# Reopen that exact workspace:
docker run --rm --init -p 127.0.0.1:8765:8765 -v seamgraph-data:/data \
  seamgraph:local serve --root /data/cache/seamgraph/repositories/PRINTED-DIRECTORY \
  --addr 0.0.0.0:8765 --allow-network
```

SeamGraph analysis remains offline. Go dependencies must be available in the
module cache or vendor tree; TypeScript resolution may require node_modules and
generated declarations. Projects that need a newer Go/Node/Python version or
specific system libraries need an adjusted runtime image. Opening a repository
does not install dependencies, run setup hooks or execute tests.

## Network boundary and validation

Inside the container the app binds to `0.0.0.0:8765` only with the explicit
`--allow-network` flag. Compose publishes it as `127.0.0.1:8765:8765` on the host.
The app retains localhost HTTP Host validation, same-origin checks and CSRF
protection. This supports local container use; it does not add authentication or
support a publicly accessible shared service. The image health check tests HTTP
liveness; it does not repeatedly analyze repositories.

CI builds the runtime image, runs `doctor`, analyzes Go/TypeScript/Python/mixed
fixtures, then checks the published HTTP model endpoint. This workspace has no
Docker/Podman executable or daemon, so the image build itself cannot be verified
here; runtime/CLI/server tests run locally and the container smoke job is prepared
for a Docker-enabled CI runner.

Docker's [multi-stage build documentation](https://docs.docker.com/build/building/multi-stage/)
explains the build/runtime stages. Its [port publishing documentation](https://docs.docker.com/engine/network/port-publishing/)
describes the explicit localhost host-port mapping used here.
