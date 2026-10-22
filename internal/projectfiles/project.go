package projectfiles

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"seamgraph/internal/application"
	"seamgraph/internal/config"
	"seamgraph/internal/ir"
	"strings"
)

const starter = `# Declare literal directory roots; adjust before scanning.
version: 1
modules:
  app:
    language: go
    paths: [internal/app]
policies:
  default: deny
  unresolved: error
`

func Initialize(root string) error {
	f, err := os.OpenFile(filepath.Join(root, config.Filename), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(starter)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func PrepareProject(root string, supplied []byte) (bool, error) {
	target := filepath.Join(root, config.Filename)
	created := false
	if supplied != nil {
		if _, err := config.Parse(supplied); err != nil {
			return false, err
		}
		if err := os.WriteFile(target, supplied, 0644); err != nil {
			return false, err
		}
	} else if _, err := os.Stat(target); os.IsNotExist(err) {
		draft, err := Discover(root)
		if err != nil {
			return false, fmt.Errorf("workspace retained at %s; cannot draft architecture: %w", root, err)
		}
		if err = os.WriteFile(target, draft, 0644); err != nil {
			return false, err
		}
		created = true
	} else if err != nil {
		return false, err
	}
	if _, err := Load(root); err != nil {
		return created, fmt.Errorf("workspace retained at %s: %w", root, err)
	}
	return created, nil
}

// Source only exposes regular, bounded files backed by observed source evidence.
func Source(root, file string, locations []ir.Evidence) (string, error) {
	if !ir.ValidPath(file) {
		return "", &application.SourceError{Kind: "invalid", Message: "invalid source location"}
	}
	known := false
	for _, location := range locations {
		known = known || location.File == file
	}
	if !known {
		return "", &application.SourceError{Kind: "missing", Message: "unknown source location"}
	}
	real, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return "", &application.SourceError{Kind: "missing", Message: "source unavailable"}
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || !ir.ValidPath(filepath.ToSlash(rel)) {
		return "", &application.SourceError{Kind: "forbidden", Message: "source escapes project"}
	}
	info, err := os.Stat(real)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 2*1024*1024 {
		return "", &application.SourceError{Kind: "large", Message: "source unavailable or too large"}
	}
	b, err := os.ReadFile(real)
	if err != nil {
		return "", &application.SourceError{Kind: "missing", Message: "source unavailable"}
	}
	var numbered strings.Builder
	for i, line := range strings.Split(string(b), "\n") {
		fmt.Fprintf(&numbered, "%5d  %s\n", i+1, line)
	}
	return numbered.String(), nil
}

func ConfigKey(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, config.Filename))
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}
