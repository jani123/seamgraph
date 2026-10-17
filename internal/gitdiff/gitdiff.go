// Package gitdiff safely extracts committed trees and compares their IR models.
// Analysis is injected; no language adapters are imported here.
package gitdiff

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

	"seamgraph/internal/gitexec"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"seamgraph/internal/snapshot"
	"seamgraph/internal/verification"
)

type Verify func(context.Context, string, ir.Snapshot, string) (verification.Report, error)
type Analyze func(context.Context, string) (ir.Snapshot, policy.Policy, error)
type Result struct {
	SchemaVersion int           `json:"schemaVersion"`
	Base          string        `json:"base"`
	Head          string        `json:"head"`
	Diff          snapshot.Diff `json:"diff"`
}

func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := gitexec.Command(ctx, root, args...)
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
	commit, err := Revision(ctx, root, ref)
	if err != nil {
		return "", "", func() {}, err
	}
	listing, err := git(ctx, root, "ls-tree", "-rz", "--full-tree", commit)
	if err != nil {
		return "", "", func() {}, err
	}
	type entry struct {
		name, object string
		mode         os.FileMode
	}
	entries := []entry{}
	objects := []string{}
	for _, record := range bytes.Split(listing, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		parts := bytes.SplitN(record, []byte{'\t'}, 2)
		if len(parts) != 2 {
			return "", "", func() {}, fmt.Errorf("invalid Git tree entry")
		}
		header := strings.Fields(string(parts[0]))
		if len(header) != 3 || (header[0] != "100644" && header[0] != "100755") || header[1] != "blob" {
			return "", "", func() {}, fmt.Errorf("symlinks/submodules or unsupported Git tree entry: %q", parts[1])
		}
		name := string(parts[1])
		if !ir.ValidPath(name) {
			return "", "", func() {}, fmt.Errorf("unsafe Git tree path %q", name)
		}
		mode := os.FileMode(0644)
		if header[0] == "100755" {
			mode = 0755
		}
		entries = append(entries, entry{name, header[2], mode})
		objects = append(objects, header[2])
	}
	cmd := gitexec.Command(ctx, root, "cat-file", "--batch")
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
	cleanup := func() { os.RemoveAll(dir) }
	fail := func(err error) (string, string, func(), error) { cleanup(); return "", "", func() {}, err }
	reader := bufio.NewReader(bytes.NewReader(data))
	for _, e := range entries {
		line, err := reader.ReadString('\n')
		if err != nil {
			return fail(err)
		}
		header := strings.Fields(line)
		if len(header) != 3 || header[0] != e.object || header[1] != "blob" {
			return fail(fmt.Errorf("invalid Git blob response"))
		}
		size, err := strconv.ParseInt(header[2], 10, 64)
		if err != nil || size < 0 || size > 1<<30 {
			return fail(fmt.Errorf("invalid or oversized Git blob"))
		}
		target := filepath.Join(dir, filepath.FromSlash(e.name))
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fail(err)
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, e.mode)
		if err != nil {
			return fail(err)
		}
		_, err = io.CopyN(f, reader, size)
		closeErr := f.Close()
		if err != nil {
			return fail(err)
		}
		if closeErr != nil {
			return fail(closeErr)
		}
		separator, err := reader.ReadByte()
		if err != nil || separator != '\n' {
			return fail(fmt.Errorf("invalid Git blob delimiter"))
		}
	}
	return dir, commit, cleanup, nil
}

func Compare(ctx context.Context, root, baseRef, headRef string, analyze Analyze, verify ...Verify) (Result, error) {
	return compare(ctx, root, baseRef, headRef, analyze, nil, verify...)
}
func compare(ctx context.Context, root, baseRef, headRef string, analyze Analyze, cache *Cache, verify ...Verify) (Result, error) {
	result := Result{SchemaVersion: 1}
	base, commit, cleanup, err := Extract(ctx, root, baseRef)
	if err != nil {
		return result, err
	}
	defer cleanup()
	result.Base = commit
	head, commit, cleanHead, err := Extract(ctx, root, headRef)
	if err != nil {
		return result, err
	}
	defer cleanHead()
	result.Head = commit
	b, bp, err := cache.analyze(ctx, result.Base, base, analyze)
	if err != nil {
		return result, fmt.Errorf("base analysis: %w", err)
	}
	h, hp, err := cache.analyze(ctx, result.Head, head, analyze)
	if err != nil {
		return result, fmt.Errorf("head analysis: %w", err)
	}
	result.Diff, err = snapshot.ComparePolicies(b, h, bp, hp)
	if err != nil {
		return result, err
	}
	if len(verify) > 0 {
		before, err := verify[0](ctx, base, b, result.Base)
		if err != nil {
			return result, err
		}
		after, err := verify[0](ctx, head, h, result.Head)
		if err != nil {
			return result, err
		}
		result.Diff.Changes = append(result.Diff.Changes, snapshot.CompareVerification(before, after)...)
	}
	snapshot.SortChanges(result.Diff.Changes)
	return result, nil
}
