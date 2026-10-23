# Repository completion rule

Before declaring implementation work complete, run `task check` from the repository
root and resolve every failure. This is required for UI changes as well as Go changes.
The gate includes the build, formatting, vet, Go race tests, the self-architecture
check, installer/development scripts, language helpers, test runners, browser tests,
and all eight Go quality tools:

- deadcode
- staticcheck
- dupl
- gocyclo
- gocognit
- golangci-lint
- gosec
- govulncheck

Run `task generate` after changing templates, embedded helpers or frontend assets.
`task quality` runs the complete Go quality gate independently; `task quality:tools`
installs its pinned tools. Devcontainers install them through `task dev:setup`.

A missing tool, scanner failure, unavailable vulnerability database, or incomplete
run is a failed check. Report the specific blocker rather than claiming the work
is complete. Do not skip a check, weaken a rule, or add broad exclusions just to
make the gate green. Any justified security suppression must name the rule and
explain the safety argument at the exact call site.

Complexity thresholds and explicit legacy function limits live in
`tools/quality/complexity.json`. New functions must meet the thresholds and existing
outliers must not grow. Lower or remove allowances when improving those functions;
do not raise limits or add allowances to accommodate newly introduced complexity.
Generated Go and test fixtures are excluded from production duplication/complexity
checks. Static analysis and race tests still cover the Go test packages.

Use the patched toolchain required by `go.mod`. `govulncheck` uses the live official
Go vulnerability database; never replace it with an empty or stale database to pass.

# Commit messages

Always use Conventional Commits: `type(scope): description`, with an optional
scope (for example, `fix: preserve Go toolchain selection` or
`ci(cache): reuse pinned tool binaries`). Use a concise, imperative description
that explains the change.

Use `feat` for new functionality and `fix` for bug fixes. Other types include
`docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, and `revert`.
Mark breaking changes with `!` before the colon or a `BREAKING CHANGE:` footer,
and explain the impact. Add a commit body when the motivation, behavior, or
validation needs more explanation than the subject provides.
