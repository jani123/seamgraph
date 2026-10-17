// Package repository materializes remote Git trees as persistent local analysis
// workspaces. Compilers use real files; callers use the returned workspace root.
package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"seamgraph/internal/gitdiff"
	"seamgraph/internal/gitexec"
)

type Workspace struct {
	Root   string `json:"root"`
	URL    string `json:"url"`
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
}

// ValidateURL accepts network transports understood by Git, including SCP-style
// SSH addresses. Local paths, remote helpers and credential-bearing HTTPS URLs
// are deliberately outside this remote-repository interface.
func ValidateURL(raw string) error {
	if strings.ContainsAny(raw, "\x00\r\n") || strings.HasPrefix(raw, "-") {
		return fmt.Errorf("invalid repository URL")
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" || u.Path == "" || u.Path == "/" {
			return fmt.Errorf("repository URL must include a host and repository path")
		}
		if u.Scheme != "https" && u.Scheme != "ssh" && u.Scheme != "git" {
			return fmt.Errorf("use an HTTPS or SSH Git repository URL")
		}
		if u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("repository URL cannot contain a query or fragment")
		}
		if u.User != nil && (u.Scheme != "ssh" || strings.Contains(u.User.String(), ":")) {
			return fmt.Errorf("use Git credentials or an SSH agent instead of credentials in the URL")
		}
		return nil
	}
	colon := strings.IndexByte(raw, ':')
	if colon <= 0 || colon == len(raw)-1 || strings.ContainsAny(raw[:colon], "/\\ ") || !strings.Contains(raw[:colon], "@") {
		return fmt.Errorf("use an HTTPS URL, ssh:// URL, or user@host:path SSH address")
	}
	if strings.HasPrefix(raw[colon+1:], "-") {
		return fmt.Errorf("invalid SSH repository path")
	}
	return nil
}

func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := gitexec.Command(ctx, dir, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return b, nil
}

// Open creates a new workspace on each invocation. It never resets or replaces
// an earlier workspace, so local config edits and installed dependencies survive.
func Open(ctx context.Context, remote, ref, cache string) (Workspace, error) {
	var w Workspace
	if err := ValidateURL(remote); err != nil {
		return w, err
	}
	if ref == "" {
		ref = "HEAD"
	}
	if strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, "\x00\r\n") {
		return w, fmt.Errorf("invalid repository revision")
	}
	if cache == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return w, err
		}
		cache = filepath.Join(base, "seamgraph", "repositories")
	}
	cache, err := filepath.Abs(cache)
	if err != nil {
		return w, err
	}
	if err = os.MkdirAll(cache, 0700); err != nil {
		return w, err
	}
	sum := sha256.Sum256([]byte(remote))
	dir, err := os.MkdirTemp(cache, hex.EncodeToString(sum[:6])+"-")
	if err != nil {
		return w, err
	}
	success := false
	defer func() {
		if !success {
			os.RemoveAll(dir)
		}
	}()
	// No checkout: materialize blobs below without filters, hooks or submodules.
	if _, err = git(ctx, "", "clone", "--no-checkout", "--no-local", "--template=", "--", remote, dir); err != nil {
		return w, err
	}
	resolved := ref
	if ref != "HEAD" {
		// A branch name refers to the remote branch; tags and commit IDs resolve as-is.
		if _, e := git(ctx, dir, "rev-parse", "--verify", "--end-of-options", "refs/remotes/origin/"+ref+"^{commit}"); e == nil {
			resolved = "refs/remotes/origin/" + ref
		}
	}
	b, err := git(ctx, dir, "rev-parse", "--verify", "--end-of-options", resolved+"^{commit}")
	if err != nil {
		return w, err
	}
	commit := strings.TrimSpace(string(b))
	// Extract reads raw Git blobs, including files excluded by export-ignore.
	// It rejects symlinks/submodules rather than following them into other trees.
	tree, _, cleanup, err := gitdiff.Extract(ctx, dir, commit)
	if err != nil {
		return w, err
	}
	defer cleanup()
	entries, err := os.ReadDir(tree)
	if err != nil {
		return w, err
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), ".git") {
			return w, fmt.Errorf("Git tree contains reserved .git path")
		}
		if err = copyTree(filepath.Join(tree, entry.Name()), filepath.Join(dir, entry.Name())); err != nil {
			return w, err
		}
	}
	if _, err = git(ctx, dir, "update-ref", "--no-deref", "HEAD", commit); err != nil {
		return w, err
	}
	if _, err = git(ctx, dir, "read-tree", commit); err != nil {
		return w, err
	}
	w = Workspace{Root: dir, URL: remote, Ref: ref, Commit: commit}
	b, err = json.MarshalIndent(w, "", "  ")
	if err != nil {
		return w, err
	}
	if err = os.WriteFile(filepath.Join(dir, ".git", "seamgraph-workspace.json"), append(b, '\n'), 0600); err != nil {
		return w, err
	}
	success = true
	return w, nil
}
func copyTree(from, to string) error {
	info, err := os.Lstat(from)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err = os.Mkdir(to, 0755); err != nil {
			return err
		}
		entries, err := os.ReadDir(from)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err = copyTree(filepath.Join(from, e.Name()), filepath.Join(to, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsupported source file: %s", from)
	}
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, info.Mode().Perm())
}
