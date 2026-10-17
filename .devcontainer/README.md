# SeamGraph Dev Container

Adapted from `../raco`: a Devbox workspace, a separate Squid allowlist proxy,
and Taskfile commands for the Dev Container CLI or VS Code/Cursor.

## Start

The host needs Docker Engine 28+ (isolated bridge gateway support), Docker
Compose, Devbox, and the Dev Container CLI. Tasks find `devcontainer` on PATH or
at `$HOME/.devcontainers/bin/devcontainer`; override with the task variable
`DEVCONTAINER=/path/to/devcontainer` if needed.

From the repository root:

```sh
devbox shell
task devcontainer:up
task devcontainer:check
task devcontainer:shell
task devcontainer:codex
# Later:
task devcontainer:codex -- resume --last
```

`up` creates the container and installs Devbox packages. `check` verifies the
toolchain, Codex, proxy allow/deny behavior, direct-egress blocking, DNS policy,
and dropped network capabilities. It requires internet access through the proxy.
To run only network checks, use `task devcontainer:check-network`.

In VS Code/Cursor choose **Reopen in Container** with its Dev Containers
extension installed. Terminals open `devbox shell`; port 8765 is forwarded.
The checkout is mounted at `/workspaces/seamgraph`. No application is started
automatically. Use `devbox run -- <command>` for noninteractive tool execution.

## Network boundary

The workspace is attached only to an internal Docker bridge with an isolated
gateway. Only the proxy has an external network. Public DNS forwarding is
disabled for the workspace, while Docker service discovery resolves `proxy`.
`NET_ADMIN` and `NET_RAW` are dropped. No Docker socket, host home, desktop
sockets, or host network is mounted.

HTTP(S) proxy variables route requests through `http://proxy:3128`. Loopback
traffic bypasses the proxy. Clients that ignore these variables cannot reach
the internet directly. Use HTTPS Git URLs; SSH is blocked.

[proxy/allowed-domains.txt](proxy/allowed-domains.txt) allows development package
registries, source downloads, tooling docs, Codex authentication/services, and
editor downloads. Exact names match only that host; names with a leading dot
also match subdomains. Squid denies unlisted hosts, numeric hosts, private and
reserved destination addresses, and ports other than HTTP 80 / CONNECT 443.
TLS is tunneled without interception, including WebSockets. Allowed services
can receive uploads; this is a destination policy, not a data-loss filter.

Policy is copied into the proxy image, which has no checkout mount and runs
read-only with dropped capabilities. Editing policy files does not change the
running policy. Review changes and run these tasks **from the host**:

```sh
task devcontainer:proxy-logs
task devcontainer:proxy-logs -- --follow
task devcontainer:rebuild
task devcontainer:check-network
```

`rebuild` builds images and recreates the workspace, interrupting active agent
sessions while retaining named volumes. Proxy denials appear as
`TCP_DENIED/403`. Add only necessary hosts, including download redirect hosts.
Logs include destinations and full plain-HTTP URLs; keep credentials out of URLs.
Host-side image builds, editor traffic, and server-side agent tools are outside
this network boundary. Host `devbox shell` retains the host's network access.

## Agent and tool state

Codex is seeded at the same version as Raco (0.162.0) in a user-owned npm prefix.
Its installation, Devbox metadata, caches, and Go modules use container-specific
named volumes. `prepare.sh` adjusts volume ownership after host UID mapping,
then enters the same Devbox environment used outside the container. There is
no alternate container Go toolchain or graphics stack.

`CODEX_HOME` is the ignored `.codex-local` directory in the checkout; host
`devbox run -- codex` and container Codex share login and session data. For a
new login use:

```sh
task devcontainer:codex -- login --device-auth
```

The launcher matches Raco: `--no-daemon` keeps process state in the container,
and `--dangerously-bypass-approvals-and-sandbox` allows autonomous work within
this container boundary. The shared checkout is writable, including deletions;
use version control/backups. Host and container sessions see different workspace
paths. To select an earlier host session:

```sh
task devcontainer:codex -- resume --all -C /workspaces/seamgraph
```

Avoid concurrent edits or generation in the same checkout. Removing containers
preserves named volumes; deleting volumes removes cached tools and downloads.
Re-enter Devbox after dependency changes. The planned application remains
loopback-bound by default; use editor port forwarding to access its UI.

## Setup validation status

Compose configuration, Taskfile discovery, JSON parsing, Bash syntax, and Python
syntax were checked during setup. Docker startup and network checks still need
`task devcontainer:check` on the host: the setup sandbox cannot access the Docker
daemon or Devbox package-resolution network. Generate and commit `devbox.lock`
with the first successful `devbox install` before implementation begins.
