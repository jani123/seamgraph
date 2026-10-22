package application

import (
	"seamgraph/internal/config"
	"testing"
)

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

func TestNormalizedPolicy(t *testing.T) {
	d, err := config.Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	p, err := NormalizePolicy(d.Config)
	if err != nil {
		t.Fatal(err)
	}
	if p.Default != "deny" || p.Unresolved != "error" || len(p.Rules) != 2 {
		t.Fatalf("incorrect normalized policy %+v", p)
	}
	if p.Rules[0].FromID != "module:npc" || p.Rules[0].ToID != "seam:world.reader" {
		t.Fatal("references not normalized", p.Rules)
	}
	d.Config.Policies.Allow[0].To = "absent"
	if _, err := NormalizePolicy(d.Config); err == nil {
		t.Fatal("invalid declaration compiled")
	}
}
