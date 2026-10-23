package gitworkspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sort"

	"seamgraph/internal/ir"
)

// workingTree captures final checkout bytes: staged, unstaged and untracked
// nonignored files. It never checks out, stages, or writes into the repository.
func workingTree(ctx context.Context, root string) (string, string, func(), error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", "", func() {}, err
	}
	scope, err := os.OpenRoot(root)
	if err != nil {
		return "", "", func() {}, err
	}
	defer func() { _ = scope.Close() }()
	listing, err := git(ctx, root, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", "", func() {}, err
	}
	paths := map[string]bool{}
	for _, p := range bytes.Split(listing, []byte{0}) {
		if len(p) > 0 {
			paths[string(p)] = true
		}
	}
	names := make([]string, 0, len(paths))
	for p := range paths {
		names = append(names, p)
	}
	sort.Strings(names)
	dir, err := os.MkdirTemp("", "seamgraph-working-")
	if err != nil {
		return "", "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	fail := func(err error) (string, string, func(), error) { cleanup(); return "", "", func() {}, err }
	hash := sha256.New()
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if err := copyWorkingFile(scope, root, dir, name, hash); err != nil {
			return fail(err)
		}
	}
	return dir, fmt.Sprintf("WORKTREE:%x", hash.Sum(nil)), cleanup, nil
}

func rejectWorkingSymlinks(root, source, name string) error {
	// Reject symlinks in every component, including directories.
	current := source
	for current != filepath.Clean(root) {
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsupported working tree symlink: %s", name)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return fmt.Errorf("path outside working tree")
		}
		current = parent
	}
	return nil
}

func copyWorkingFile(scope *os.Root, root, dir, name string, hash hash.Hash) error {
	if !ir.ValidPath(name) {
		return fmt.Errorf("unsafe working tree path %q", name)
	}
	source := filepath.Join(root, filepath.FromSlash(name))
	if err := rejectWorkingSymlinks(root, source, name); err != nil {
		return err
	}
	info, err := os.Lstat(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsupported working tree file: %s", name)
	}
	data, err := scope.ReadFile(name)
	if err != nil {
		return err
	}
	target := filepath.Join(dir, filepath.FromSlash(name))
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	// #nosec G703 -- ir.ValidPath rejects traversal; the destination is a new private temporary tree.
	if err = os.WriteFile(target, data, info.Mode().Perm()); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(hash, "%s\x00%d\x00%d\x00", name, info.Mode().Perm()&0111, len(data))
	hash.Write(data)
	return nil
}
