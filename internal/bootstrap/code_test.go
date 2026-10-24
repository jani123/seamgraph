package bootstrap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCodeWithoutArchitectureOrGit(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":  "module example.test/code\n\ngo 1.26.0\n",
		"code.go": "package code\n// Read returns a value.\nfunc Read() int {return 1}\nfunc caller() int {return Read()}\n// Store holds state.\ntype Store struct { hidden int }\nfunc (s Store) Get() int {return Read()}\ntype Derived Store\n",
		"code.ts": "/** TypeScript documentation. */\nexport function read(){return 1}\nexport function caller(){return read()}\nexport class Store {private hidden=1}\n",
		"code.py": "def read():\n    \"\"\"Python documentation.\"\"\"\n    return 1\ndef caller(): return read()\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	browser, err := service.Code(context.Background(), "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(browser.Issues) != 0 {
		t.Fatal(browser.Issues)
	}
	if len(browser.Files) != 4 {
		t.Fatal(browser.Files)
	}
	seen := map[string]bool{}
	for _, object := range browser.Objects {
		definition := object.Symbol.Definition
		if definition == nil {
			continue
		}
		if object.Symbol.ModuleID != "module:unassigned" || len(object.Seams) != 0 {
			t.Fatal(object)
		}
		if definition.Name == "hidden" && definition.Location.File == "code.go" {
			if definition.ParentID == "" || definition.Documentation != "" {
				t.Fatal("incorrect field declaration", definition)
			}
		}
		if definition.Name != "Read" && definition.Name != "read" {
			continue
		}
		if definition.Documentation == "" || definition.Digest == "" {
			t.Fatal(definition)
		}
		seen[definition.Location.File] = true
		selected, err := service.Code(context.Background(), "", "", "", object.Symbol.ID)
		if err != nil || selected.Selected != definition.Location.File || selected.Content.Text == "" {
			t.Fatal(selected, err)
		}
		found := false
		for _, relation := range browser.Relations {
			if relation.ToID == object.Symbol.ID && relation.FromID != "module:unassigned" && relation.Evidence[0].Kind == "call" {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing local caller: %s", object.Symbol.ID)
		}
	}
	if len(seen) != 3 {
		t.Fatal(seen)
	}
	for _, name := range []string{"seamgraph.yaml", ".git"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatal("created", name, err)
		}
	}
}

func TestCodeKeepsDeclaredIdentities(t *testing.T) {
	for _, language := range []string{"go", "typescript", "python"} {
		t.Run(language, func(t *testing.T) {

			root := copyProject(t, language)
			service, err := New(root)
			if err != nil {
				t.Fatal(err)
			}
			graph, _, err := Scan(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			browser, err := service.Code(context.Background(), "", "", "", "")
			if err != nil || len(browser.Issues) != 0 {
				t.Fatal(browser.Issues, err)
			}
			objects := map[string]bool{}
			for _, object := range browser.Objects {
				objects[object.Symbol.ID] = true
			}
			for _, symbol := range graph.Symbols {
				if !objects[symbol.ID] {
					t.Fatalf("lost architecture identity %s", symbol.ID)
				}
			}
			for _, seam := range graph.Seams {
				for _, id := range seam.ContractMembers {
					found := false
					for _, object := range browser.Objects {
						if object.Symbol.ID == id && slices.Contains(object.Seams, seam.ID) {
							found = true
						}
					}
					if !found {
						t.Fatalf("lost seam context %s", id)
					}
				}
			}
		})
	}
}

func TestCodeRevisionChangesAndRemovedSource(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.test", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.test")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatal(string(output), err)
		}
		return strings.TrimSpace(string(output))
	}
	write := func(source string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "code.py"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write("def read(): return 1\ndef removed(): return read()\n")
	git("add", ".")
	git("commit", "-qm", "feat: base")
	base := git("rev-parse", "HEAD")
	write("def read(): return 2\n")
	git("commit", "-qam", "fix: change body")
	head := git("rev-parse", "HEAD")
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	browser, err := service.Code(context.Background(), head, base, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(browser.Objects) != 2 {
		t.Fatal(browser.Objects)
	}
	var removed string
	for _, object := range browser.Objects {
		if object.Symbol.Definition.Name == "read" && object.Status != "modified" {
			t.Fatal(object)
		}
		if object.Symbol.Definition.Name == "removed" {
			if object.Status != "removed" {
				t.Fatal(object)
			}
			removed = object.Symbol.ID
		}
	}
	selected, err := service.Code(context.Background(), head, base, "", removed)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Revision != head || selected.ContentRevision != base || selected.RelationshipRevision != base || !strings.Contains(selected.Content.Text, "def removed") {
		t.Fatal(selected)
	}
	// Live files invalidate the content cache without changing immutable revisions.
	write("def read(): return 3\n")
	live, err := service.Code(context.Background(), "", "", "", "")
	if err != nil || live.Revision != "Working directory" || len(live.Objects) != 1 {
		t.Fatal(live, err)
	}
	if live.Objects[0].Symbol.Definition.Digest == browser.Objects[0].Symbol.Definition.Digest {
		t.Fatal("stale live index")
	}
}
