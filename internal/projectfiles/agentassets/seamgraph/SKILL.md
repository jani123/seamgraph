---
name: seamgraph
description: Explore and verify project architecture with SeamGraph. Use when investigating module boundaries or consumers, changing public interfaces or cross-module dependencies, moving code between modules, or reviewing architecture violations and refactors in a SeamGraph-configured project.
---
<!-- seamgraph:managed-skill -->

# SeamGraph architecture workflow

SeamGraph statically analyzes Go, TypeScript, and Python using project-defined
modules and public interfaces called seams. It reports observed relations,
source evidence, and policy violations. Use it to guide architecture work;
source review and the project's normal tests remain necessary.

## Establish context

Run commands from the project root. Use `{{command}}` for this project.
Read `seamgraph.yaml` for declared module roots, public seams, and dependency
policies. If it is missing, explain that analysis needs configuration; use
`init` only when project setup is requested, then review its starter boundaries.
Do not replace an existing configuration.

Use `{{command}} --help` and `<command> --help` for the installed version's
options. When analysis reports missing tools, use `{{command}} doctor` to
identify runtime requirements; use the project's dependency setup to resolve them.

## Explore and investigate

Start with `{{command}} scan` for a summary. Use
`{{command}} scan --json` when you need module, seam, symbol, or relation IDs.
Reuse returned IDs rather than guessing them.

Inspect relevant entities and their relations with
`{{command}} inspect <id> --json`. Follow evidence locations into source to
understand callers, consumers, and ownership before changing dependencies.
Static relations are observed source references, not proof of runtime execution
or complete behavior coverage. Respect diagnostics and analysis limitations.

For violations, run `{{command}} check --json`, inspect the affected entities,
and assess whether the implementation or the declared architecture needs to
change. Preserve the intended boundaries; do not relax policies, suppress
findings, or add baseline entries merely to make the check pass.

## Verify architecture changes

After changing public interfaces, module ownership, or cross-module dependencies,
run `{{command}} check --json` alongside the project's normal tests. In a Git
project, use `{{command}} diff --base HEAD --head WORKTREE --json` to review
uncommitted architecture changes. For branch reviews, select the actual review
base revision instead of assuming HEAD covers the whole branch.

Exit code 0 means success, 1 means policy/verification failure, and 2 means an
operational failure or incomplete analysis. Read the diagnostics and report
incomplete results instead of treating missing observations as a clean check.

When the task concerns declared contracts and test evidence, use
`{{command}} inspect <id> --tests --json` and
`{{command}} check --contracts --json`. Contract verification depends on the
project's manifest and fresh collected evidence; a static scan alone does not
establish that behavior has been tested. Running `test` executes project tests;
`scan`, `inspect`, and `check` do not collect new execution evidence.

Report relevant IDs, evidence locations, architecture changes, and check results
at the level needed for the task. Use `serve` when the user wants the visual
explorer.
