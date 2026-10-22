package projectfiles

const valid = `# architecture
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
  allow:
    - from: npc
      to: world.reader
    - from: scripts
      to: world.reader
`
