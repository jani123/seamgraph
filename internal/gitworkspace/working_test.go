package gitworkspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWorkingTreeCapturesFinalBytesWithoutChangingIndex(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		b, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(b))
		}
		return string(b)
	}
	write := func(name, value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write("tracked.py", "base")
	write("gone.py", "gone")
	write(".gitignore", "ignored.py\n")
	git("add", ".")
	git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	write("tracked.py", "staged")
	git("add", "tracked.py")
	write("tracked.py", "final")
	write("new.ts", "new")
	write("ignored.py", "ignore")
	os.Remove(filepath.Join(root, "gone.py"))
	status := git("status", "--porcelain=v1")
	index := git("show", ":tracked.py")
	tree, id, cleanup, err := Extract(context.Background(), root, "WORKTREE")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for name, want := range map[string]string{"tracked.py": "final", "new.ts": "new"} {
		b, err := os.ReadFile(filepath.Join(tree, name))
		if err != nil || string(b) != want {
			t.Fatal(name, string(b), err)
		}
	}
	for _, name := range []string{"gone.py", "ignored.py", ".git"} {
		if _, err := os.Stat(filepath.Join(tree, name)); !os.IsNotExist(err) {
			t.Fatal("unexpected capture", name, err)
		}
	}
	if git("status", "--porcelain=v1") != status || git("show", ":tracked.py") != index {
		t.Fatal("comparison modified checkout/index")
	}
	write("tracked.py", "later")
	_, next, cleanNext, err := Extract(context.Background(), root, "WORKTREE")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanNext()
	if id == next {
		t.Fatal("working cache identity did not change")
	}
	if err := os.Symlink("tracked.py", filepath.Join(root, "link.py")); err != nil {
		t.Fatal(err)
	}
	if _, _, cleanup, err := Extract(context.Background(), root, "WORKTREE"); err == nil {
		cleanup()
		t.Fatal("symlink accepted")
	}
}
