package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIAgentSetup(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "devbox.json"), []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	for i := range 2 {
		out.Reset()
		if code := run([]string{"agent", "setup", "--root", root}, &out, &diagnostics); code != 0 {
			t.Fatalf("setup: %d %s", code, diagnostics.String())
		}
		if i == 1 && !strings.Contains(out.String(), "already up to date") {
			t.Fatal(out.String())
		}
	}
	for _, path := range []string{"AGENTS.md", ".agents/skills/seamgraph/SKILL.md"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || !bytes.Contains(data, []byte("devbox run seamgraph")) {
			t.Fatalf("%s: %s %v", path, data, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "seamgraph.yaml")); !os.IsNotExist(err) {
		t.Fatal("setup should not create architecture configuration")
	}
}

func TestCLIAgentSetupErrorsAndHelp(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("<!-- seamgraph:begin -->\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := run([]string{"agent", "setup", "--root", root}, &out, &diagnostics); code != 2 || !strings.Contains(diagnostics.String(), "markers") {
		t.Fatalf("%d %s", code, diagnostics.String())
	}
	out.Reset()
	diagnostics.Reset()
	if code := run([]string{"agent", "setup", "--help"}, &out, &diagnostics); code != 0 || !strings.Contains(out.String(), "--root") {
		t.Fatalf("%d %s %s", code, out.String(), diagnostics.String())
	}
}
