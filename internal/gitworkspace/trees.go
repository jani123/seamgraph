// Package gitworkspace owns exact revision trees and remote workspaces.
package gitworkspace

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"seamgraph/internal/ir"
)

func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := Command(ctx, root, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git: %w: %s", err, stderr.String())
	}
	return b, nil
}
func Revision(ctx context.Context, root, ref string) (string, error) {
	b, err := git(ctx, root, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	return strings.TrimSpace(string(b)), err
}

// Extract reads exact tree blobs, bypassing export-ignore/export-subst archive
// attributes. No Git hook, checkout, working tree or source file is modified.
func Extract(ctx context.Context, root, ref string) (string, string, func(), error) {
	if ref == "WORKTREE" {
		return workingTree(ctx, root)
	}
	commit, err := Revision(ctx, root, ref)
	if err != nil {
		return "", "", func() {}, err
	}
	listing, err := git(ctx, root, "ls-tree", "-rz", "--full-tree", commit)
	if err != nil {
		return "", "", func() {}, err
	}
	entries, objects, err := parseTreeEntries(listing)
	if err != nil {
		return "", "", func() {}, err
	}
	cmd := Command(ctx, root, "cat-file", "--batch")
	cmd.Stdin = strings.NewReader(strings.Join(objects, "\n") + "\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return "", "", func() {}, fmt.Errorf("read Git blobs: %w: %s", err, stderr.String())
	}
	dir, err := os.MkdirTemp("", "seamgraph-revision-")
	if err != nil {
		return "", "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	fail := func(err error) (string, string, func(), error) { cleanup(); return "", "", func() {}, err }
	reader := bufio.NewReader(bytes.NewReader(data))
	for _, e := range entries {
		if err := writeTreeBlob(dir, reader, e); err != nil {
			return fail(err)
		}
	}
	return dir, commit, cleanup, nil
}

type treeEntry struct {
	name, object string
	mode         os.FileMode
}

func parseTreeEntries(listing []byte) ([]treeEntry, []string, error) {
	entries := []treeEntry{}
	objects := []string{}
	for _, record := range bytes.Split(listing, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		parts := bytes.SplitN(record, []byte{'\t'}, 2)
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("invalid Git tree entry")
		}
		header := strings.Fields(string(parts[0]))
		if len(header) != 3 || (header[0] != "100644" && header[0] != "100755") || header[1] != "blob" {
			return nil, nil, fmt.Errorf("symlinks/submodules or unsupported Git tree entry: %q", parts[1])
		}
		name := string(parts[1])
		if !ir.ValidPath(name) {
			return nil, nil, fmt.Errorf("unsafe Git tree path %q", name)
		}
		mode := os.FileMode(0644)
		if header[0] == "100755" {
			mode = 0755
		}
		entries = append(entries, treeEntry{name, header[2], mode})
		objects = append(objects, header[2])
	}
	return entries, objects, nil
}

func writeTreeBlob(dir string, reader *bufio.Reader, e treeEntry) error {
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	header := strings.Fields(line)
	if len(header) != 3 || header[0] != e.object || header[1] != "blob" {
		return fmt.Errorf("invalid Git blob response")
	}
	size, err := strconv.ParseInt(header[2], 10, 64)
	if err != nil || size < 0 || size > 1<<30 {
		return fmt.Errorf("invalid or oversized Git blob")
	}
	target := filepath.Join(dir, filepath.FromSlash(e.name))
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	// #nosec G304 -- ir.ValidPath rejects traversal; regular Git blobs are written into a new private temporary tree.
	f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, e.mode)
	if err != nil {
		return err
	}
	_, err = io.CopyN(f, reader, size)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	separator, err := reader.ReadByte()
	if err != nil || separator != '\n' {
		return fmt.Errorf("invalid Git blob delimiter")
	}
	return nil
}
