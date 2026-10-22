package gitworkspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
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
	cleanup := func() { os.RemoveAll(dir) }
	fail := func(err error) (string, string, func(), error) { cleanup(); return "", "", func() {}, err }
	hash := sha256.New()
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if !ir.ValidPath(name) {
			return fail(fmt.Errorf("unsafe working tree path %q", name))
		}
		source := filepath.Join(root, filepath.FromSlash(name))
		// Reject symlinks in every component, including directories.
		current := source
		for current != filepath.Clean(root) {
			info, err := os.Lstat(current)
			if os.IsNotExist(err) {
				break
			}
			if err != nil {
				return fail(err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fail(fmt.Errorf("unsupported working tree symlink: %s", name))
			}
			parent := filepath.Dir(current)
			if parent == current {
				return fail(fmt.Errorf("path outside working tree"))
			}
			current = parent
		}
		info, err := os.Lstat(source)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fail(err)
		}
		if !info.Mode().IsRegular() {
			return fail(fmt.Errorf("unsupported working tree file: %s", name))
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return fail(err)
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fail(err)
		}
		if err = os.WriteFile(target, data, info.Mode().Perm()); err != nil {
			return fail(err)
		}
		fmt.Fprintf(hash, "%s\x00%d\x00%d\x00", name, info.Mode().Perm()&0111, len(data))
		hash.Write(data)
	}
	return dir, fmt.Sprintf("WORKTREE:%x", hash.Sum(nil)), cleanup, nil
}
