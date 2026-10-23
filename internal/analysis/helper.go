package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"seamgraph/internal/config"
	"seamgraph/internal/ir"
)

// RunHelper invokes an embedded adapter with a versioned protocol. Helpers never
// execute application code. Runtime availability and protocol versions are checked.
func RunHelper(ctx context.Context, root string, c config.Config, executable, filename string, files map[string][]byte, flags ...string) (ir.PartialSnapshot, error) {
	var partial ir.PartialSnapshot
	path, err := exec.LookPath(executable)
	if err != nil {
		return partial, fmt.Errorf("%s is required for this adapter: %w", executable, err)
	}
	dir, err := os.MkdirTemp("", "seamgraph-helper-")
	if err != nil {
		return partial, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	for name, data := range files {
		p := filepath.Join(dir, name)
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return partial, err
		}
		if err = os.WriteFile(p, data, 0600); err != nil {
			return partial, err
		}
	}
	req := struct {
		Version int           `json:"version"`
		Root    string        `json:"root"`
		Config  config.Config `json:"config"`
	}{1, root, c}
	b, err := json.Marshal(req)
	if err != nil {
		return partial, err
	}
	// #nosec G204 -- Executable and script are fixed by embedded adapters; arguments are passed without a shell.
	cmd := exec.CommandContext(ctx, path, append(flags, filepath.Join(dir, filename))...)
	for _, v := range os.Environ() {
		key, _, _ := strings.Cut(v, "=")
		if key != "NODE_OPTIONS" && key != "NODE_PATH" && key != "PYTHONPATH" && key != "PYTHONHOME" && key != "PYTHONSTARTUP" {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(b)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return partial, fmt.Errorf("%s analyzer failed: %w: %s", executable, err, stderr.String())
	}
	var response struct {
		Version  int         `json:"version"`
		Snapshot ir.Snapshot `json:"snapshot"`
	}
	if err = json.Unmarshal(output, &response); err != nil {
		return partial, fmt.Errorf("invalid helper output: %w", err)
	}
	if response.Version != 1 || response.Snapshot.SchemaVersion != ir.SchemaVersion {
		return partial, fmt.Errorf("unsupported helper protocol/schema version")
	}
	s := response.Snapshot
	// Complete referential integrity is checked after merging all language adapters.
	partial = ir.PartialSnapshot{Modules: s.Modules, Seams: s.Seams, Symbols: s.Symbols, Relations: s.Relations, Diagnostics: s.Diagnostics}
	return partial, nil
}
