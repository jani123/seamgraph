package projectfiles

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentSetupRepeatAndUpgrade(t *testing.T) {
	root := t.TempDir()
	prefix, suffix := "# Project rules\n\nKeep this rule.\n\n", "\n\n## More rules\nKeep these too.\n"
	original := prefix + agentBegin + "\nOld guidance\n" + agentEnd + suffix
	putAgentFile(t, root, "AGENTS.md", original)
	putAgentFile(t, root, skillPath, "---\nname: seamgraph\n---\n"+skillOwner+"\nOld skill\n")
	if err := os.Chmod(filepath.Join(root, "AGENTS.md"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := SetupAgent(root)
	if err != nil || len(changed) != 2 {
		t.Fatalf("upgrade: %v %v", changed, err)
	}
	instructions := readSetupFile(t, root, "AGENTS.md")
	if !strings.HasPrefix(instructions, prefix) || !strings.HasSuffix(instructions, suffix) || strings.Contains(instructions, "Old guidance") || strings.Count(instructions, agentBegin) != 1 {
		t.Fatalf("instructions not preserved: %s", instructions)
	}
	skill := readSetupFile(t, root, skillPath)
	info, err := os.Stat(filepath.Join(root, "AGENTS.md"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("mode changed: %v %v", info, err)
	}
	changed, err = SetupAgent(root)
	if err != nil || len(changed) != 0 {
		t.Fatalf("repeat rewrote files: %v %v", changed, err)
	}
	if instructions != readSetupFile(t, root, "AGENTS.md") || skill != readSetupFile(t, root, skillPath) {
		t.Fatal("repeat changed bytes")
	}
}

func TestAgentSetupNewProjectAndDevbox(t *testing.T) {
	for _, config := range []string{"", "devbox.json", "devbox.jsonc"} {
		t.Run(config, func(t *testing.T) {
			root := t.TempDir()
			if config != "" {
				putAgentFile(t, root, config, "{}\n")
			}
			changed, err := SetupAgent(root)
			if err != nil || len(changed) != 2 {
				t.Fatalf("setup: %v %v", changed, err)
			}
			for _, name := range []string{"AGENTS.md", skillPath} {
				text := readSetupFile(t, root, name)
				if strings.Contains(text, "{{command}}") || strings.Contains(text, "devbox run seamgraph") != (config != "") {
					t.Fatalf("wrong command in %s: %s", name, text)
				}
			}
			changed, err = SetupAgent(root)
			if err != nil || len(changed) != 0 {
				t.Fatalf("repeat: %v %v", changed, err)
			}
			if config != "" {
				if err := os.Remove(filepath.Join(root, config)); err != nil {
					t.Fatal(err)
				}
				changed, err = SetupAgent(root)
				if err != nil || len(changed) != 2 || strings.Contains(readSetupFile(t, root, skillPath), "devbox run") {
					t.Fatalf("command refresh: %v %v", changed, err)
				}
			}
		})
	}
}

func TestAgentSetupRejectsMalformedMarkers(t *testing.T) {
	for _, text := range []string{
		agentBegin, agentEnd, agentEnd + "\n" + agentBegin,
		agentBegin + "\n" + agentBegin + "\n" + agentEnd,
		agentBegin + "\n" + agentEnd + "\n" + agentEnd,
		"prefix " + agentBegin + "\n" + agentEnd,
		agentBegin + "\n" + agentEnd + " suffix",
	} {
		t.Run(text, func(t *testing.T) {
			root := t.TempDir()
			putAgentFile(t, root, "AGENTS.md", text)
			changed, err := SetupAgent(root)
			if err == nil || len(changed) != 0 {
				t.Fatalf("accepted malformed markers: %v %v", changed, err)
			}
			if readSetupFile(t, root, "AGENTS.md") != text {
				t.Fatal("changed malformed instructions")
			}
			if _, err := os.Stat(filepath.Join(root, skillPath)); !os.IsNotExist(err) {
				t.Fatalf("created skill: %v", err)
			}
		})
	}
}

func TestAgentSetupRejectsUnmanagedSkillBeforeWriting(t *testing.T) {
	root := t.TempDir()
	putAgentFile(t, root, "AGENTS.md", "Project rules\n")
	putAgentFile(t, root, skillPath, "My custom skill\n")
	_, err := SetupAgent(root)
	if err == nil {
		t.Fatal("overwrote unmanaged skill")
	}
	if readSetupFile(t, root, "AGENTS.md") != "Project rules\n" || readSetupFile(t, root, skillPath) != "My custom skill\n" {
		t.Fatal("conflict changed files")
	}
}

func TestAgentSetupRejectsSymlinks(t *testing.T) {
	for _, name := range []string{"AGENTS.md", ".agents"} {
		t.Run(name, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			target := outside
			if name == "AGENTS.md" {
				target = filepath.Join(outside, "rules")
				putAgentFile(t, outside, "rules", "Outside rules\n")
			}
			if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
				t.Skip(err)
			}
			if _, err := SetupAgent(root); err == nil {
				t.Fatal("accepted symlink outside project")
			}
			entries, err := os.ReadDir(outside)
			if err != nil {
				t.Fatal(err)
			}
			if name == ".agents" && len(entries) != 0 {
				t.Fatal("wrote outside project")
			}
			if name == "AGENTS.md" && readSetupFile(t, outside, "rules") != "Outside rules\n" {
				t.Fatal("changed outside rules")
			}
		})
	}
}

func TestMergeAgentRulesPreservesSurroundingBytes(t *testing.T) {
	for _, original := range []string{"rules without newline", "rules\n", "rules\r\n", "\xff\n", "prefix\r\n" + agentBegin + "\r\nold\r\n" + agentEnd + "\r\nsuffix"} {
		updated, err := mergeAgentRules([]byte(original), agentRules)
		if err != nil {
			t.Fatal(err)
		}
		repeated, err := mergeAgentRules(updated, agentRules)
		if err != nil || !bytes.Equal(updated, repeated) {
			t.Fatalf("not idempotent: %q %v", original, err)
		}
		if !strings.Contains(original, agentBegin) && !bytes.HasPrefix(updated, []byte(original)) {
			t.Fatalf("changed prefix: %q", original)
		}
		if strings.HasSuffix(original, "suffix") && (!bytes.HasPrefix(updated, []byte("prefix\r\n")) || !bytes.HasSuffix(updated, []byte("\r\nsuffix"))) {
			t.Fatal("changed surrounding CRLF bytes")
		}
	}
}

func putAgentFile(t *testing.T, root, name, text string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

func readSetupFile(t *testing.T, root, name string) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}
