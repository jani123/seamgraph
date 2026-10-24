<!-- seamgraph:begin -->
## Architecture with SeamGraph

SeamGraph analyzes module boundaries, public interfaces ("seams"), dependencies,
and architecture violations in Go, TypeScript, and Python. Configuration lives
in `seamgraph.yaml`.

Use SeamGraph when exploring unfamiliar modules, changing public interfaces or
cross-module dependencies, moving code between modules, or reviewing architecture
violations and refactors. Read `.agents/skills/seamgraph/SKILL.md` for the workflow.

- Discover commands: `{{command}} --help`
- Summarize architecture: `{{command}} scan`
- Inspect an entity: `{{command}} inspect <id> --json`
- Check architecture rules: `{{command}} check --json`
- Review uncommitted changes: `{{command}} diff --base HEAD --head WORKTREE --json`

Read `seamgraph.yaml` before interpreting results. Discover IDs from
`scan --json`; use reported source evidence to investigate findings. After
changing boundaries, public interfaces, or cross-module dependencies, run the
architecture check alongside the project's tests. Report incomplete analysis
and runtime failures. Do not weaken rules or expand baselines merely to pass.
<!-- seamgraph:end -->
