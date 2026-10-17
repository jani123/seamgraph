package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"seamgraph/internal/repository"
)

func TestOpenPreparesDiscoverableRemote(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("SSH transport stand-in is POSIX-only")
	}
	source := t.TempDir()
	for name, body := range map[string]string{"world/api.py": "VALUE = 1\n", "app/main.py": "from world.api import VALUE\n"} {
		p := filepath.Join(source, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-b", "main"}, {"add", "."}, {"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fixture"}} {
		if b, e := exec.Command("git", append([]string{"-C", source}, args...)...).CombinedOutput(); e != nil {
			t.Fatalf("git: %s %v", b, e)
		}
	}
	script := filepath.Join(t.TempDir(), "ssh-fixture")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexec git-upload-pack \"$SEAMGRAPH_TEST_REMOTE\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_SSH_COMMAND", script)
	t.Setenv("GIT_SSH_VARIANT", "ssh")
	t.Setenv("SEAMGRAPH_TEST_REMOTE", source)
	var out, errs bytes.Buffer
	if code := run([]string{"open", "git@example.test:project.git", "--prepare-only", "--json", "--cache-dir", t.TempDir()}, &out, &errs); code != 0 {
		t.Fatalf("%d: %s", code, errs.String())
	}
	var w repository.Workspace
	if err := json.Unmarshal(out.Bytes(), &w); err != nil {
		t.Fatal(err)
	}
	if w.Commit == "" || w.Root == "" {
		t.Fatal("missing workspace identity")
	}
	out.Reset()
	errs.Reset()
	if code := run([]string{"scan", "--root", w.Root, "--json"}, &out, &errs); code != 0 {
		t.Fatalf("scan %d: %s", code, errs.String())
	}
	if !strings.Contains(out.String(), "module:world") || !strings.Contains(out.String(), "module:app") || !strings.Contains(out.String(), `"kind": "uses"`) {
		t.Fatalf("did not analyze discovered architecture: %s", out.String())
	}
	// An explicit local configuration replaces inference in the new workspace.
	supplied := filepath.Join(t.TempDir(), "custom.yaml")
	content := "version: 1\nmodules:\n  project:\n    language: python\n    paths: [.]\npolicies:\n  default: deny\n"
	if err := os.WriteFile(supplied, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errs.Reset()
	if code := run([]string{"open", "git@example.test:project.git", "--config", supplied, "--prepare-only", "--json", "--cache-dir", t.TempDir()}, &out, &errs); code != 0 {
		t.Fatalf("%d: %s", code, errs.String())
	}
	if err := json.Unmarshal(out.Bytes(), &w); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filepath.Join(w.Root, "seamgraph.yaml"))
	if err != nil || string(actual) != content {
		t.Fatalf("config was not copied verbatim: %s %v", actual, err)
	}
}

func TestDoctorReportsMissingRuntimesWithoutProject(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var out, errs bytes.Buffer
	if code := run([]string{"doctor", "--json"}, &out, &errs); code != 2 {
		t.Fatalf("missing runtime returned %d: %s", code, out.String())
	}
	var tools []struct {
		Name  string `json:"name"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &tools); err != nil {
		t.Fatal(err)
	}
	if len(tools) != 5 {
		t.Fatalf("got %d runtime checks", len(tools))
	}
	for _, tool := range tools {
		if tool.Error == "" {
			t.Fatalf("%s missing but reported ready", tool.Name)
		}
	}
}

func TestDoctorOnlyChecksConfiguredLanguages(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("runtime stand-in uses POSIX shell")
	}
	tools := t.TempDir()
	if err := os.WriteFile(filepath.Join(tools, "python3"), []byte("#!/bin/sh\necho 'Python 3.12.0'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools)
	root := t.TempDir()
	content := "version: 1\nmodules:\n  app:\n    language: python\n    paths: [app]\npolicies:\n  default: allow\n"
	if err := os.WriteFile(filepath.Join(root, "seamgraph.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := run([]string{"doctor", "--root", root, "--json"}, &out, &errs); code != 0 {
		t.Fatalf("%d %s %s", code, errs.String(), out.String())
	}
	var toolsFound []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out.Bytes(), &toolsFound); err != nil {
		t.Fatal(err)
	}
	if len(toolsFound) != 1 || toolsFound[0].Name != "python3" {
		t.Fatalf("checked unrelated runtimes: %s", out.String())
	}
}
