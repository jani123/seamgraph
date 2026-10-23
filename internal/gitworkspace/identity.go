// Identity conservatively fingerprints project inputs for evidence freshness.
package gitworkspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func Identity(ctx context.Context, root string) (string, string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	scope, err := os.OpenRoot(root)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = scope.Close() }()
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "runtime:%s:%s:%s\x00", runtime.GOOS, runtime.GOARCH, runtime.Version())
	for _, key := range []string{"GOOS", "GOARCH", "GOFLAGS", "CGO_ENABLED", "GOWORK", "GOTOOLCHAIN", "NODE_OPTIONS", "PYTHONPATH", "PYTHONHASHSEED", "PYTHONOPTIMIZE", "VIRTUAL_ENV"} {
		_, _ = fmt.Fprintf(h, "env:%s=%s\x00", key, os.Getenv(key))
	}
	ignored := map[string]bool{".git": true, ".seamgraph": true, "node_modules": true, ".build": true, ".venv": true, "__pycache__": true, ".devbox": true, ".codex-local": true, ".pytest_cache": true, ".mypy_cache": true, ".ruff_cache": true}
	err = filepath.WalkDir(root, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if file != root && ignored[d.Name()] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("freshness fingerprint cannot safely follow symlink %s", rel)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported project file %s", rel)
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00", filepath.ToSlash(rel), info.Size())
		f, err := scope.Open(rel)
		if err != nil {
			return err
		}
		_, err = io.Copy(h, f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		h.Write([]byte{0})
		return closeErr
	})
	if err != nil {
		return "", "", err
	}
	fingerprint := hex.EncodeToString(h.Sum(nil))
	cmd := Command(ctx, root, "rev-parse", "--verify", "HEAD")
	b, err := cmd.Output()
	if err != nil {
		return "", fingerprint, nil
	}
	return strings.TrimSpace(string(b)), fingerprint, nil
}
