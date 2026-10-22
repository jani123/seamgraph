package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"seamgraph/internal/application"
	"seamgraph/internal/projectfiles"
	"strings"
	"testing"

	"seamgraph/internal/config"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
)

func copyProject(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join("../../testdata/projects", name)
	err := filepath.WalkDir(source, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(source, p)
		target := filepath.Join(root, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func TestSharedAdapterContract(t *testing.T) {
	for _, name := range []string{"go", "typescript", "python"} {
		t.Run(name, func(t *testing.T) {
			root := copyProject(t, name)
			s, c, err := Scan(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Validate(); err != nil {
				t.Fatal(err)
			}
			if len(s.Modules) != 2 || len(s.Seams) != 1 || len(s.Seams[0].ContractMembers) == 0 || len(s.Relations) == 0 {
				t.Fatal("incomplete adapter model", s)
			}
			p, _ := application.NormalizePolicy(c)
			r, err := policy.Evaluate(s, p)
			if err != nil || r.ExitCode() != 0 {
				t.Fatalf("allowed reference failed: %+v %v", r, err)
			}
			b, err := s.JSON()
			if err != nil {
				t.Fatal(err)
			}
			again, _, err := Scan(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			next, _ := again.JSON()
			if string(b) != string(next) {
				t.Fatal("nondeterministic scan")
			}
		})
	}
}
func TestBypassesAndUncertainty(t *testing.T) {
	for _, tc := range []struct{ name, file, bad, dynamic string }{
		{"typescript", "npc/main.ts", "import {open} from '../world/internal'; export const bad = open();", "const name = './runtime'; export const uncertain = import(name);"},
		{"python", "src/npc/main.py", "from world.internal import secret\nbad = secret()\n", "import importlib\nimportlib.import_module('world.' + input())\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := copyProject(t, tc.name)
			for _, input := range []struct {
				source string
				want   int
			}{{tc.bad, 1}, {tc.dynamic, 2}} {
				if err := os.WriteFile(filepath.Join(root, tc.file), []byte(input.source), 0644); err != nil {
					t.Fatal(err)
				}
				s, c, err := Scan(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				p, _ := application.NormalizePolicy(c)
				r, err := policy.Evaluate(s, p)
				if err != nil || r.ExitCode() != input.want {
					t.Fatalf("got %+v err=%v want exit=%d", r, err, input.want)
				}
			}
		})
	}
}
func TestMixedLanguageMerge(t *testing.T) {
	root := t.TempDir()
	var configs []string
	for _, name := range []string{"go", "typescript", "python"} {
		source := copyProject(t, name)
		target := filepath.Join(root, name)
		if err := os.Rename(source, target); err != nil {
			t.Fatal(err)
		}
		switch name {
		case "go":
			configs = append(configs, "  gw:\n    language: go\n    paths: [go/world]\n  gn:\n    language: go\n    paths: [go/npc]\n")
		case "typescript":
			configs = append(configs, "  tw:\n    language: typescript\n    paths: [typescript/world]\n  tn:\n    language: typescript\n    paths: [typescript/npc]\n")
		case "python":
			configs = append(configs, "  pw:\n    language: python\n    paths: [python/src/world]\n    importRoots: [python/src]\n  pn:\n    language: python\n    paths: [python/src/npc]\n    importRoots: [python/src]\n")
		}
	}
	// A workspace enables the nested Go module without changing its sources.
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.25.0\nuse ./go\n"), 0644); err != nil {
		t.Fatal(err)
	}
	data := "version: 1\nmodules:\n" + strings.Join(configs, "") + "policies:\n  default: allow\n  allow:\n    - from: pn\n      to: gw\n"
	if err := os.WriteFile(filepath.Join(root, "seamgraph.yaml"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	s, _, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Modules) != 6 {
		t.Fatal("mixed project missing modules", s.Modules)
	}
	for _, r := range s.Relations {
		if r.FromID == "module:pn" && r.ToID == "module:gw" {
			t.Fatal("declared cross-language intent fabricated observed reference")
		}
	}
}

func TestWorkingMixedProject(t *testing.T) {
	root := copyProject(t, "mixed")
	s, c, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	p, err := application.NormalizePolicy(c)
	if err != nil {
		t.Fatal(err)
	}
	r, err := policy.Evaluate(s, p)
	if err != nil || r.ExitCode() != 0 {
		t.Fatal("mixed configured project failed", err, r)
	}
	if len(s.Modules) != 6 || len(s.Seams) != 3 {
		t.Fatal("missing mixed modules/seams")
	}
	for _, edge := range s.Relations {
		if ir.Owners(s)[edge.FromID] == "module:py-scripts" && ir.Owners(s)[edge.ToID] == "module:go-world" {
			t.Fatal("fabricated cross-language edge")
		}
	}
}

func TestMixedPublicAndDeepImportsRemainViolations(t *testing.T) {
	for _, tc := range []struct{ name, file, source string }{{"typescript", "npc/main.ts", "import {open} from '@world';\nimport {secret} from '../world/internal';\nexport const bad = open().read() + secret();\n"}, {"python", "src/npc/main.py", "from world.api import open_world\nimport world.internal\nbad = world.internal.secret()\n"}} {
		t.Run(tc.name, func(t *testing.T) {
			root := copyProject(t, tc.name)
			if err := os.WriteFile(filepath.Join(root, tc.file), []byte(tc.source), 0644); err != nil {
				t.Fatal(err)
			}
			s, c, err := Scan(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			p, _ := application.NormalizePolicy(c)
			r, err := policy.Evaluate(s, p)
			if err != nil || r.ExitCode() != 1 {
				t.Fatal("known mixed-path bypass should violate, not fail analysis or pass", err, r)
			}
			for _, v := range r.Violations {
				if v.Code == "SEAM_BYPASS" && v.Evidence[0].StartLine == 1 {
					t.Fatal("permitted import wrongly used as bypass evidence", v)
				}
			}
		})
	}
}

func TestLiteralProjectRootForAllAdapters(t *testing.T) {
	for _, tc := range []struct{ language, file, source string }{{"go", "api.go", "package api\nfunc Read() string{return \"world\"}\n"}, {"typescript", "api.ts", "export function read(): string { return 'world' }\n"}, {"python", "api.py", "def read() -> str:\n    return 'world'\n"}} {
		t.Run(tc.language, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, tc.file), []byte(tc.source), 0644); err != nil {
				t.Fatal(err)
			}
			if tc.language == "go" {
				if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/api\ngo 1.25.0\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			cfg := "version: 1\nmodules:\n  app:\n    language: " + tc.language + "\n    paths: [.]\npolicies:\n  default: deny\n"
			if err := os.WriteFile(filepath.Join(root, "seamgraph.yaml"), []byte(cfg), 0644); err != nil {
				t.Fatal(err)
			}
			s, c, err := Scan(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			p, _ := application.NormalizePolicy(c)
			r, err := policy.Evaluate(s, p)
			if err != nil || r.ExitCode() != 0 || len(s.Modules) != 1 || len(s.Symbols) == 0 {
				t.Fatal("project-root module failed", err, r, s)
			}
		})
	}
}
func TestScopedAndAliasedDynamicPythonAreExplicit(t *testing.T) {
	for _, source := range []string{"def read():\n    from world.internal import secret\n    return secret()\n", "import importlib as loader\nloader.import_module('world.' + input())\n"} {
		root := copyProject(t, "python")
		if err := os.WriteFile(filepath.Join(root, "src/npc/main.py"), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
		s, c, err := Scan(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		p, _ := application.NormalizePolicy(c)
		r, err := policy.Evaluate(s, p)
		if err != nil || r.ExitCode() != 2 {
			t.Fatal("unsupported runtime/scoped reference silently accepted", err, r)
		}
	}
}
func TestGoDeepPackageImportIsBypass(t *testing.T) {
	root := copyProject(t, "go")
	if err := os.MkdirAll(filepath.Join(root, "world/private"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "world/private/private.go"), []byte("package private\nfunc Hidden(){}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "npc/deep.go"), []byte("package npc\nimport _ \"example.test/architecture/world/private\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	s, c, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := application.NormalizePolicy(c)
	r, err := policy.Evaluate(s, p)
	if err != nil || r.ExitCode() != 1 || len(r.Violations) == 0 {
		t.Fatal("side-effect deep import silently permitted", err, r)
	}
	for _, v := range r.Violations {
		if v.Code == "SEAM_BYPASS" && v.Evidence[0].File != "npc/deep.go" {
			t.Fatal("permitted source wrongly cited", v)
		}
	}
}

func TestPythonStarAndNamespaceReexports(t *testing.T) {
	for _, tc := range []struct{ api, consumer string }{{"from .internal import *\n", "from world.api import open_world\nvalue = open_world()\n"}, {"from . import internal as operations\n__all__ = ['operations']\n", "from world.api import operations\nvalue = operations.open_world()\n"}} {
		root := copyProject(t, "python")
		if err := os.WriteFile(filepath.Join(root, "src/world/api.py"), []byte(tc.api), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "src/world/__init__.py"), []byte("from .api import *\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "src/npc/main.py"), []byte(tc.consumer), 0644); err != nil {
			t.Fatal(err)
		}
		s, c, err := Scan(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		p, _ := application.NormalizePolicy(c)
		r, err := policy.Evaluate(s, p)
		if err != nil || r.ExitCode() != 0 {
			t.Fatal("static Python reexport failed", err, r)
		}
	}
}

func TestUnownedLocalReferencesAreNeverSilentlyAllowed(t *testing.T) {
	for _, name := range []string{"go", "typescript", "python"} {
		t.Run(name, func(t *testing.T) {
			root := copyProject(t, name)
			doc, err := projectfiles.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			delete(doc.Config.Modules, "world")
			for _, mode := range []string{"error", "warn"} {
				doc.Config.Policies = config.Policies{Default: "allow", Unresolved: mode}
				b, err := doc.Config.YAML()
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(root, config.Filename), b, 0644); err != nil {
					t.Fatal(err)
				}
				s, c, err := Scan(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				p, _ := application.NormalizePolicy(c)
				r, err := policy.Evaluate(s, p)
				if err != nil {
					t.Fatal(err)
				}
				want := 2
				if mode == "warn" {
					want = 0
				}
				if r.ExitCode() != want || len(r.Diagnostics) == 0 {
					t.Fatal("missing owner silently treated as allowed", name, mode, r)
				}
			}
		})
	}
}

func Scan(ctx context.Context, root string) (ir.Snapshot, config.Config, error) {
	return scanner().Scan(ctx, root)
}
