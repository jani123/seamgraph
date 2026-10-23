# Opening Git repositories

Run SeamGraph on your workstation first. The browser UI stays on loopback and Git
uses your workstation's SSH agent, known hosts and credential helpers. A separate
hosted service is unnecessary for trying public and private repositories.

For a packaged runtime with all analysis tools included, see [container deployment](deployment.md).

## Install and run

From the SeamGraph checkout:

```sh
go install ./cmd/seamgraph
```

Put Go's binary directory on PATH, or run `.build/seamgraph` after `task build`.
Git is required for remote repositories. Keep Go 1.26.10+ available for Go analysis,
Node 18+ for TypeScript, and Python 3.10+ for Python. The compiler helper, Python
helper and browser assets are embedded; no frontend build is needed at runtime.

```sh
seamgraph open https://github.com/pallets/itsdangerous.git
seamgraph open git@github.com:your-team/your-project.git --ref main
seamgraph open ssh://git@git.example.com:2222/team/project.git --ref v1.2.0
```

Open http://127.0.0.1:8765. Put the repository URL before the flags. `--ref` accepts
a branch, tag or commit present in the clone; the default is the remote HEAD.
Branches are resolved against origin. Full history is fetched so committed
revision comparison works. SSH uses the installed Git/SSH clients and your
existing configuration; SeamGraph does not collect or copy private keys. Unlock
keys in your SSH agent and establish normal host trust before opening a private
repository. HTTPS uses configured Git credentials; passwords/tokens in URLs are
rejected. No push operation is performed.

## Managed filesystem

Each `open` creates a fresh directory under the OS user cache directory,
`seamgraph/repositories/`. Override the parent with `--cache-dir /path/to/workspaces`.
The terminal prints the workspace path and exact resolved commit. Metadata is
stored in `.git/seamgraph-workspace.json`.

The repository layer materializes exact Git blobs into ordinary local files. This
provides an isolated repository root while retaining the filesystem paths that
Go packages, the TypeScript compiler and Python import resolution need. It is not
a FUSE mount or an in-memory filesystem. Git history is retained locally; hooks,
checkout filters and submodule initialization are not run during materialization.
Git symlinks and submodules are currently rejected explicitly, as in revision diff.
Git LFS objects are not downloaded; their pointer blobs remain ordinary files.

Closing the server leaves the workspace intact. Reopen an existing workspace with:

```sh
seamgraph serve --root /printed/workspace/path
```

A new `open` creates another workspace at the requested remote revision. It never
resets an existing workspace or overwrites earlier config edits. Remove unused
workspace directories yourself when finished; there is no automatic eviction or
shared mutable mirror cache yet.

## Repositories without SeamGraph configuration

If the repository contains `seamgraph.yaml`, it is used as-is. Otherwise SeamGraph
writes an **exploratory** configuration: source directories become disjoint
modules; Python `src` import roots are recognized; dependency/build/cache folders
are skipped. A directory containing source files owns its subtree. Sources at the
repository root can therefore produce one module for the whole repository. Mixed
languages sharing such a directory require an explicit configuration.

Discovery uses default allow and unresolved warnings. It declares no seams and
makes no architectural enforcement claim. Review the generated module roots,
define public seams, then tighten policy. The graph can contain isolated modules
when there are no observed imports between the discovered roots. Discovery is a
starting point, not automatic inference of a project's intended architecture.

Supply a config kept outside the source repository with:

```sh
seamgraph open git@github.com:your-team/project.git --config ./project-seamgraph.yaml
```

That file is copied verbatim into the new workspace. The remote is not changed.
Committed revision comparisons use each commit's own configuration; an uncommitted
generated or supplied config does not retroactively define historical architecture.

## Prepare dependencies or edit configuration first

```sh
seamgraph open https://github.com/owner/project.git --prepare-only --json
# Use the root printed in the JSON result:
cd /printed/workspace/path
# Edit seamgraph.yaml and install project dependencies as needed.
seamgraph serve --root .
```

Opening fetches the repository; analysis itself remains offline. It does not run
package installation, project setup scripts or tests. Go dependencies must be in
the module cache or vendor tree; TypeScript projects may need node_modules and
Python projects may need their intended import roots configured. Missing analysis
inputs are reported in the graph panel. Test execution stays an explicit CLI
operation.

For container use, the Git/SSH clients, runtime tools, dependency caches and agent
socket must be available inside that container. The native workstation deployment
avoids that extra setup. A shared hosted deployment with tenant isolation and
server-side credential management is separate work; this command is a local app.

## Validation

Integration tests fetch a local Git daemon, check branch/tag resolution, preserve
raw blobs despite export-ignore/filter attributes, verify clean Git state and
retain earlier edits. A local upload-pack stand-in exercises Git's SSH transport;
private-host authentication is left to the installed Git/SSH setup. CLI acceptance
checks prepare a remote workspace, generate config, analyze cross-module Python
references and honor a supplied configuration. A public HTTPS smoke test fetched
`pallets/itsdangerous` and scanned the generated workspace successfully.

Git behavior follows the [clone](https://git-scm.com/docs/git-clone),
[configuration](https://git-scm.com/docs/git-config), and
[read-tree](https://git-scm.com/docs/git-read-tree) documentation.
