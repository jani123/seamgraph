package config

import (
	"strings"
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

func TestRoundTrip(t *testing.T) {
	d, err := Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if d.Config.Policies.Unresolved != "error" {
		t.Fatal("missing strict default")
	}
	b, err := d.Config.YAML()
	if err != nil {
		t.Fatal(err)
	}
	d2, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := d2.Config.YAML()
	if string(b) != string(again) {
		t.Fatal("unstable round trip")
	}
	if d.Node.Content[0].Content[0].HeadComment != "# architecture" {
		t.Fatal("comment lost from syntax tree")
	}
	modules, seams := d.Config.IRModules()
	if len(modules) != 4 || len(seams) != 3 {
		t.Fatal("declarations missing")
	}
}
func TestOwnership(t *testing.T) {
	d, err := Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]string{"internal/world/read.go": "module:world", "internal/world/sub/read.go": "module:world", "internal/worldly/read.go": "", "README.md": ""} {
		got, err := d.Config.Owner(file)
		if err != nil || got != want {
			t.Errorf("%s: %q %v", file, got, err)
		}
	}
	for _, file := range []string{"/etc/passwd", "../secret", "internal/../secret", `C:\secret`, "src\\ui\\index.ts"} {
		if _, err := d.Config.Owner(file); err == nil {
			t.Errorf("unsafe path accepted %q", file)
		}
	}
}
func TestInvalidConfig(t *testing.T) {
	tests := map[string]string{
		"empty": "", "malformed": "version: [", "wrong version": strings.Replace(valid, "version: 1", "version: 2", 1),
		"unknown field": valid + "typo: true\n", "unknown nested field": strings.Replace(valid, "language: go", "langauge: go", 1),
		"duplicate key": valid + "version: 1\n", "duplicate module": strings.Replace(valid, "  npc:", "  world:", 1),
		"unknown seam":        strings.Replace(valid, "to: world.reader", "to: world.missing", 1),
		"unknown consumer":    strings.Replace(valid, "from: npc", "from: absent", 1),
		"unknown module":      strings.Replace(valid, "to: world.reader", "to: missing", 1),
		"overlap":             strings.Replace(valid, "internal/npc", "internal/world/sub", 1),
		"same root":           strings.Replace(valid, "internal/npc", "internal/world", 1),
		"same module nesting": strings.Replace(valid, "[internal/world]", "[internal/world, internal/world/sub]", 1),
		"glob":                strings.Replace(valid, "internal/npc", "internal/*", 1),
		"escape":              strings.Replace(valid, "internal/npc", "../npc", 1),
		"absolute":            strings.Replace(valid, "internal/npc", "/npc", 1),
		"unclean":             strings.Replace(valid, "internal/npc", "internal/./npc", 1),
		"no roots":            strings.Replace(valid, "[internal/npc]", "[]", 1),
		"language":            strings.Replace(valid, "language: go", "language: rust", 1),
		"module name":         strings.Replace(valid, "  npc:", "  npc.bad:", 1),
		"seam name":           strings.Replace(valid, "      reader:", "      reader.bad:", 1),
		"empty seam":          strings.Replace(valid, "symbols: [WorldReader]", "symbols: []", 1),
		"duplicate symbols":   strings.Replace(valid, "[WorldReader]", "[WorldReader, WorldReader]", 1),
		"outside entrypoint":  strings.Replace(valid, "[src/ui/index.ts]", "[src/scripts/api.py]", 1),
		"go entrypoint":       strings.Replace(valid, "symbols: [WorldReader]", "entrypoints: [internal/world/api.go]", 1),
		"default":             strings.Replace(valid, "default: deny", "default: maybe", 1),
		"unresolved":          strings.Replace(valid, "default: deny", "default: deny\n  unresolved: ignore", 1),
		"multiple documents":  valid + "---\nversion: 1\n",
		"null":                strings.Replace(valid, "symbols: [WorldReader]", "symbols: null", 1),
		"alias":               strings.Replace(valid, "paths: [internal/npc]", "paths: &paths [internal/npc]", 1) + "extra: *paths\n",
		"conflicting rules":   valid + "  deny:\n    - from: npc\n      to: world.reader\n",
	}
	for n, input := range tests {
		t.Run(n, func(t *testing.T) {
			if _, err := Parse([]byte(input)); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestNormalizedPolicy(t *testing.T) {
	d, err := Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Config.Policy()
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
	if _, err := d.Config.Policy(); err == nil {
		t.Fatal("invalid declaration compiled")
	}
}

func TestProjectRootOwnership(t *testing.T) {
	doc, err := Parse([]byte("version: 1\nmodules:\n  app:\n    language: go\n    paths: [.]\npolicies:\n  default: deny\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"main.go", "internal/world/read.go"} {
		id, err := doc.Config.Owner(file)
		if err != nil || id != "module:app" {
			t.Fatal(file, id, err)
		}
	}
	c := doc.Config
	c.Modules["nested"] = Module{Language: "go", Paths: []string{"internal/nested"}}
	if err = c.Validate(); err == nil {
		t.Fatal("root module overlapped nested module")
	}
}
