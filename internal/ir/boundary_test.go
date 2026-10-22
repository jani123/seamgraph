package ir_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// This test also covers future downstream packages, before adapters are added.
func TestArchitectureImportBoundary(t *testing.T) {
	protected := map[string]bool{"ir": true, "core": true, "policy": true, "rules": true, "snapshot": true, "application": true, "analysis": true, "config": true, "diff": true, "server": true, "web": true, "ui": true, "verification": true}
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel("..", path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if !protected[parts[0]] {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			top := parts[0]
			pure := top == "ir" || top == "policy" || top == "snapshot" || top == "verification" || top == "config"
			if (pure || top == "application" || top == "server" || top == "ui") && (p == "os" || strings.HasPrefix(p, "os/") || p == "syscall") {
				t.Errorf("%s owns no filesystem/process capability: %s", path, p)
			}
			if (pure || top == "application") && p == "io/fs" {
				t.Errorf("%s imports filesystem capability %s", path, p)
			}
			if pure && (p == "net" || (strings.HasPrefix(p, "net/") && p != "net/url")) {
				t.Errorf("pure module %s imports network capability %s", path, p)
			}
			if (top == "application" || top == "server" || top == "ui" || pure) && (p == "seamgraph/internal/projectfiles" || p == "seamgraph/internal/gitworkspace" || p == "seamgraph/internal/bootstrap") {
				t.Errorf("%s bypasses injected capabilities with %s", path, p)
			}
			if strings.HasPrefix(p, "seamgraph/internal/analyzer") || strings.HasPrefix(p, "golang.org/x/tools") || strings.HasPrefix(p, "go/") || strings.Contains(p, "typescript") || strings.Contains(p, "python") {
				t.Errorf("%s imports language-specific package %s", path, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
