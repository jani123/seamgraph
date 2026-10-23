package projectfiles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"seamgraph/internal/application"
	"seamgraph/internal/ir"
)

func TestSourceConfinementAndBoundedReads(t *testing.T) {
	root := t.TempDir()
	write := func(name, value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	known := func(file string) []ir.Evidence { return []ir.Evidence{{File: file}} }
	write("source.go", "package source\n")
	if data, err := Source(root, "source.go", known("source.go")); err != nil || !strings.Contains(data, "1  package source") {
		t.Fatal(data, err)
	}
	if _, err := Source(root, "source.go", nil); err == nil {
		t.Fatal("source without evidence was exposed")
	}
	write("large.go", strings.Repeat("a", 2*1024*1024+1))
	_, err := Source(root, "large.go", known("large.go"))
	var sourceError *application.SourceError
	if !errors.As(err, &sourceError) || sourceError.Kind != "large" {
		t.Fatal("oversized source exposed", err)
	}
	outside := filepath.Join(t.TempDir(), "secret.go")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err = Source(root, "escape.go", known("escape.go"))
	if !errors.As(err, &sourceError) || sourceError.Kind != "forbidden" {
		t.Fatal("source escaped project root", err)
	}
	if err := os.Symlink("source.go", filepath.Join(root, "inside.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := Source(root, "inside.go", known("inside.go")); err != nil {
		t.Fatal("in-project source link rejected", err)
	}
}
