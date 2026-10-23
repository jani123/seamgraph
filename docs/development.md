# Development server with Air

The primary workflow uses the Dev Container CLI. Run these on the host:

```sh
# Once after updating the container configuration:
task devcontainer:rebuild
# Ensure Air is running, print the browser URL and follow logs:
task devcontainer:dev
# Print that URL again from another host terminal:
task devcontainer:url
```

The host needs Python 3 for the URL helper, plus the existing Dev Container CLI,
Docker/Compose and Task/Devbox setup. The helper discovers the running workspace
by Dev Container labels, reads its actual Compose project label, and finds the
published relay port. It does not guess container names or Compose project names.

Container startup automatically launches one background Air session and waits
for HTTP readiness. Repeated starts reuse that session. Logs are written to
`.build/dev-server.log`. To stop it, run `task devcontainer:dev-stop` on the host
or `python3 .devcontainer/start-dev.py --stop` inside the container.

Inside an already prepared container, stop the background session before using
`task dev` to run Air in the foreground with custom arguments.
Existing containers can run `task dev:setup` once to install development tools.

New devcontainers run `task dev:setup` during post-create. Air v1.65.3 is installed
in `.build/bin`, separately from application dependencies. This version supports
the project's Go 1.26.10 toolchain. No global Air installation is needed.

By default the dev server explores SeamGraph itself. For another project:

```sh
task dev -- --root testdata/projects/mixed
# Or use an environment variable:
SEAMGRAPH_DEV_ROOT=/workspaces/another-project task dev
```

Air regenerates Templ, compiler/Python helpers and frontend assets before building.
It watches Go, template, helper and asset source; dependency trees, temporary
outputs, generated templates and generated bundles are excluded. Refresh the
browser after changes to load the newly built UI. Air's optional browser proxy
is not used.

## Dynamic host ports with the standalone CLI

Within the devcontainer, Air serves on `0.0.0.0:8765` with explicit container
network binding. The workspace remains attached only to its isolated private
network. A small immutable Nginx relay can reach that fixed workspace endpoint
and publishes its listener on **127.0.0.1 at an available host port**. It forwards
the original Host and Origin headers, retaining the application's Host/CSRF
checks. It is a fixed-target HTTP ingress, separate from the Squid egress proxy.

Docker selects the host port when creating the relay container. It stays stable
through Air rebuilds and server restarts, and may change when the relay is
recreated. Multiple project containers can all use internal port 8765 without
colliding on the host. The relay has no checkout mount and does not give the
workspace an internet connection or a default route.

Open the URL printed by `task devcontainer:dev` or `task devcontainer:url` on the
host. A 502 response means the relay is running but Air has not finished starting,
or the backend has stopped. Ctrl-C closes the log stream while Air continues
running. `task devcontainer:dev-stop` stops the background server.

VS Code support remains optional: process-based port discovery can still forward
the server and label it **SeamGraph dev**. The CLI workflow uses Docker's published
port and requires no editor port-forwarding service.

## Native development without a devcontainer

Outside a devcontainer, `task dev` uses `127.0.0.1:0` and the OS selects a free
port. A per-session file under `.build` keeps the address stable across Air
rebuilds. A new session chooses a new port. To select one explicitly:

```sh
SEAMGRAPH_DEV_ADDR=127.0.0.1:9876 task dev
```

For a one-shot server with an OS-assigned port:

```sh
go run ./cmd/seamgraph serve --root . --addr 127.0.0.1:0
```

`--listen-file /path/to/file` optionally records the actual bound `host:port` for
local tooling. It is written only after a successful bind. Development uses its
own temporary file; normal serving does not create one.

The native Air smoke check observed two rebuilds on the same selected port.
Unit tests cover Docker label/project discovery, dynamic host ports, missing or
ambiguous containers and container/native launcher arguments. Docker-enabled CI
builds the relay, serves the mixed graph from an isolated backend, resolves its
host URL using the real helper, and verifies Host checks through the relay. There
is no Docker daemon in this workspace, so that ingress integration cannot be run
locally here.

References: [Air configuration](https://github.com/air-verse/air/blob/v1.65.3/air_example.toml),
[Docker port publishing](https://docs.docker.com/engine/network/port-publishing/),
and the [Dev Container port attributes schema](https://github.com/devcontainers/spec/blob/main/schemas/devContainer.base.schema.json).

## Required completion checks

Run `task check` before considering a change finished, including changes to the UI.
The repository's [completion rule](../AGENTS.md) requires the full suite: build,
formatting, vet, Go race tests, architecture, installer/development scripts, language
helpers, runner integration, browser interactions, and the Go quality gate.
Run `task generate` first when changing templates, helpers or frontend assets.

```sh
task quality:tools # Install pinned tools; also part of dev:setup.
task quality       # Run all eight Go quality tools.
task check         # Required full completion gate.
```

[versions.json](../tools/quality/versions.json) pins deadcode, staticcheck, dupl,
gocyclo, gocognit, golangci-lint, gosec and govulncheck. Tools are installed into
`.build/quality/bin` separately from application dependencies. Go installations use
module checksum verification; golangci-lint uses its official release binary with
the release SHA-256 checksums. No global tool installation is needed. The installer
supports Linux/macOS on amd64/arm64. The current Devbox lock supplies Go 1.26.9 as a
bootstrap; Go automatically downloads and selects Go 1.26.10 as required by `go.mod`.
Keep automatic toolchain selection enabled when using that bootstrap.

The gate checks every module package, not incidental Go files in `.build` or nested
fixture modules. Staticcheck and golangci-lint include test packages; deadcode uses
test entry points too. Duplication and complexity checks cover manually written
production Go. Generated Templ Go and test setup are excluded from these structural
metrics. Dupl's threshold is 150 tokens, cyclomatic complexity is 20 and cognitive
complexity is 30. [complexity.json](../tools/quality/complexity.json) sets these limits
with no legacy function allowances. Every manually written production function
must meet both complexity thresholds. No baseline permits dead code, lint,
duplication, security or vulnerability findings. Golangci-lint runs errcheck, govet,
ineffassign and unused; the other requested analyzers run independently.

Reports and an aggregate result are written to `.build/quality/reports`. Deadcode
and dupl findings fail the gate even when the tool itself exits zero. Scanner
errors and missing tools also fail; the runner attempts all eight checks and
returns failure if any check fails. CI runs the same gate on pull requests and
before releases and retains acceptance-job reports.

Govulncheck queries the live [Go vulnerability database](https://go.dev/doc/security/vuln/)
through its [official GCS bucket](https://storage.googleapis.com/go-vulndb/index/db.json),
which is reachable through the existing devcontainer egress allowlist. `GOVULNDB`
can select another maintained database endpoint. This check requires network
access on every run; an unavailable database is a failure. It runs separately from
CI's offline build/test step. Go 1.26.10 replaces the vulnerable 1.25 build toolchain;
see [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809) for the latest required fix.
