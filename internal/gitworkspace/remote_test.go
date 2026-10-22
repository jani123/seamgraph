package gitworkspace

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}
func remoteFixture(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	src := filepath.Join(base, "project")
	if err := os.Mkdir(src, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, src, "init", "-b", "main")
	runGit(t, src, "config", "user.name", "Fixture")
	runGit(t, src, "config", "user.email", "fixture@example.invalid")
	for name, body := range map[string]string{"app.py": "VALUE = 1\n", ".gitattributes": "app.py filter=trap\nkeep.txt export-ignore\n", "keep.txt": "exact blob\n"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "script.sh"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, src, "add", ".")
	runGit(t, src, "commit", "-qm", "initial")
	runGit(t, src, "tag", "v1")
	first := runGit(t, src, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(src, "app.py"), []byte("VALUE = 2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, src, "commit", "-qam", "second")
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	cmd := exec.Command("git", "daemon", "--reuseaddr", "--export-all", "--base-path="+base, "--listen=127.0.0.1", fmt.Sprintf("--port=%d", port), base)
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	for i := 0; i < 100; i++ {
		c, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Millisecond*50)
		if e == nil {
			c.Close()
			return fmt.Sprintf("git://127.0.0.1:%d/project", port), first
		}
		time.Sleep(time.Millisecond * 10)
	}
	t.Fatal("git daemon did not start")
	return "", ""
}
func TestOpenRemote(t *testing.T) {
	remote, first := remoteFixture(t)
	cache := t.TempDir()
	// A smudge filter would corrupt the file or execute arbitrary checkout code.
	marker := filepath.Join(t.TempDir(), "filter-ran")
	global := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(global, []byte("[filter \"trap\"]\n smudge = touch "+marker+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	// Parent Git variables cannot redirect extraction to the wrong repository.
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "nonexistent"))
	w, err := Open(context.Background(), remote, "v1", cache)
	if err != nil {
		t.Fatal(err)
	}
	if w.Commit != first {
		t.Fatalf("tag resolved to %s, wanted %s", w.Commit, first)
	}
	for name, want := range map[string]string{"app.py": "VALUE = 1\n", "keep.txt": "exact blob\n"} {
		b, err := os.ReadFile(filepath.Join(w.Root, name))
		if err != nil || string(b) != want {
			t.Fatalf("%s: %q %v", name, b, err)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("checkout filter executed")
	}
	// Tests' git helper must not inherit our deliberately invalid GIT_DIR.
	if err := os.Unsetenv("GIT_DIR"); err != nil {
		t.Fatal(err)
	}
	if got := runGit(t, w.Root, "status", "--porcelain"); got != "" {
		t.Fatalf("dirty materialization: %s", got)
	}
	if err := os.WriteFile(filepath.Join(w.Root, "seamgraph.yaml"), []byte("local edits"), 0644); err != nil {
		t.Fatal(err)
	}
	latest, err := Open(context.Background(), remote, "main", cache)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Root == w.Root || latest.Commit == first {
		t.Fatal("new invocation did not isolate its revision")
	}
	b, _ := os.ReadFile(filepath.Join(w.Root, "seamgraph.yaml"))
	if string(b) != "local edits" {
		t.Fatal("overwrote previous workspace")
	}
	if _, err = Open(context.Background(), remote, "missing-branch", cache); err == nil {
		t.Fatal("accepted missing ref")
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 2 {
		t.Fatalf("failed workspace not cleaned: %d %v", len(entries), err)
	}
}
func TestValidateURL(t *testing.T) {
	for _, s := range []string{"https://github.com/acme/repo.git", "git@github.com:acme/repo.git", "ssh://git@example.com:2222/project.git", "git://localhost/project"} {
		if err := ValidateURL(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	for _, s := range []string{"", "/tmp/project", "file:///tmp/project", "ext::sh -c evil", "--upload-pack=evil", "https://user:secret@example.com/r", "https://example.com/r?token=secret", "git@host:-bad", "ssh://host/"} {
		if err := ValidateURL(s); err == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}

// Exercise Git's SSH transport with a local upload-pack stand-in; no private
// host or credentials are needed in the test suite.
func TestSSHTransport(t *testing.T) {
	remote, _ := remoteFixture(t)
	base, err := Open(context.Background(), remote, "HEAD", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if os.PathSeparator == '\\' {
		t.Skip("shell transport fixture is POSIX-only")
	}
	script := filepath.Join(t.TempDir(), "ssh-fixture")
	log := filepath.Join(t.TempDir(), "ssh-args")
	if err = os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SEAMGRAPH_TEST_SSH_LOG\"\nexec git-upload-pack \"$SEAMGRAPH_TEST_REMOTE\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_SSH_COMMAND", script)
	t.Setenv("GIT_SSH_VARIANT", "ssh")
	t.Setenv("SEAMGRAPH_TEST_REMOTE", base.Root)
	t.Setenv("SEAMGRAPH_TEST_SSH_LOG", log)
	got, err := Open(context.Background(), "git@example.test:team/project.git", "HEAD", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got.Commit != base.Commit {
		t.Fatal("SSH transport returned a different tree")
	}
	args, err := os.ReadFile(log)
	if err != nil || !strings.Contains(string(args), "git@example.test") {
		t.Fatalf("SSH transport not invoked: %s %v", args, err)
	}
}
