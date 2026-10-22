package projectfiles

import (
	"os"
	"path/filepath"
	"seamgraph/internal/config"
	"strings"
	"testing"
)

func TestDiscover(t *testing.T) {
	for _, tc := range []struct {
		name    string
		files   []string
		modules int
		wantErr string
	}{
		{"mixed project", []string{"go/world/world.go", "go/npc/main.go", "typescript/ui/index.ts", "python/src/app/main.py"}, 4, ""},
		{"root owns subtree", []string{"main.go", "internal/world/world.go"}, 1, ""},
		{"ambiguous languages", []string{"main.go", "ui/index.ts"}, 0, "mixed languages"},
		{"no sources", []string{"README.md"}, 0, "no Go"},
		{"excluded dependencies", []string{"src/world/index.ts", "src/npc/index.ts", "node_modules/thing/index.ts", ".venv/lib/x.py"}, 2, ""},
		{"colliding names", []string{"a-b/x.py", "a/b/x.py"}, 2, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, p := range tc.files {
				full := filepath.Join(root, p)
				if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
			}
			b, err := Discover(root)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			d, err := config.Parse(b)
			if err != nil {
				t.Fatal(err)
			}
			if len(d.Config.Modules) != tc.modules {
				t.Fatalf("got %s", b)
			}
			if d.Config.Policies.Default != "allow" || d.Config.Policies.Unresolved != "warn" {
				t.Fatal("discovery should be exploratory")
			}
			for _, m := range d.Config.Modules {
				if len(m.Seams) != 0 {
					t.Fatal("invented seams")
				}
			}
			second, err := Discover(root)
			if err != nil || string(second) != string(b) {
				t.Fatal("non-deterministic discovery")
			}
		})
	}
}
