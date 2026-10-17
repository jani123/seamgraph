# Configuration v1

Configuration lives at `<root>/seamgraph.yaml`. The parser uses yaml.v3, rejects
unknown fields at every schema level, duplicate mapping keys, nulls, aliases,
merge keys, malformed YAML, and multiple documents. Comments remain in the node
tree. The browser edits only a targeted rule list after preview and confirmation.
Aliases are deliberately unsupported to keep edits explicit and avoid expansion
surprises.

```yaml
version: 1
modules:
  world:
    language: go
    paths: [internal/world]
    seams:
      reader:
        symbols: [WorldReader]
  npc:
    language: go
    paths: [internal/npc]
  scripts:
    language: python
    paths: [src/scripts]
    importRoots: [src]
    seams:
      public:
        entrypoints: [src/scripts/api.py]
  ui:
    language: typescript
    paths: [src/ui]
    seams:
      public:
        entrypoints: [src/ui/index.ts]
policies:
  default: deny
  unresolved: error
  allow:
    - from: npc
      to: world.reader
    - from: scripts
      to: world.reader
```

Module and seam names contain ASCII letters, digits, underscores or hyphens.
Names are case-sensitive. Roots are clean relative slash paths, never absolute,
parent-traversing paths. `paths: [.]` means the whole project directory and cannot
coexist with another module root. Literal roots only: glob patterns fail explicitly.
Every pair of roots must be disjoint, including roots in the same module. Nested
module ownership has no implicit precedence. `internal/worldly` is not a child
of `internal/world`. Unmatched source paths are explicitly unowned.

`validate` validates declarations without requiring files to exist. Scanning requires roots to exist and rejects source symlinks. Adapters enforce
source containment.
Callers of `Owner` must supply normalized project-relative paths after resolving
symlinks; it is a lexical ownership operation, not a file access authorization API.

Languages are `go`, `typescript`, and `python`. A seam needs nonempty `symbols`
and/or `entrypoints`. Go uses symbols only; TS/Python can use both. Entrypoints
must lie inside the owning roots; Python entrypoints must end in `.py`.
`importRoots` is available only for Python. Selectors are declarations, not proof
of resolvable symbols; adapters must diagnose missing or ambiguous selectors.
No seam membership is inferred merely from the presence of an exported symbol.

Policies require `default: allow|deny`; `unresolved: error|warn` defaults to
`error`. `allow` and `deny` contain `{from: module, to: module[.seam]}` entries.
All references must exist. Identical or conflicting source/target pairs fail
validation; distinct module and seam rules can coexist. Explicit deny wins over matching grants. A module grant permits dependency
intent; seam grants permit references to their members. A cross-language
declaration records intent, never an observed dependency. CLI checks enforce the shared policy core. See [policy semantics](policy-and-diff.md).

Glob precedence, optional nested ownership and configuration aliases/migrations
require explicit future designs. Go uses the selected
Go build environment. Contract mappings are described in [contracts](contracts.md).

TypeScript modules may set `tsconfig: path/to/tsconfig.json`; the path is relative
to the project. Otherwise configuration is discovered above the configured source
roots. A shared tsconfig is currently required for all TS roots in one scan.

The browser rule editor preserves unrelated config bytes, serializes only the
selected allow/deny list, previews a diff, and checks for concurrent edits before
atomic replacement. It saves `seamgraph.yaml.bak`. Block-style policies mappings
are required for safe editing; flow-style policies fail without writing. Exact
duplicate/conflicting rule pairs are rejected, including from browser proposals.
