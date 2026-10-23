package projectfiles

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
	// #nosec G304 -- Local project initialization uses a constant filename with exclusive creation.
	f, err := os.OpenFile(filepath.Join(root, config.Filename), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
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
		if err := os.WriteFile(target, supplied, 0600); err != nil {
			return false, err
		}
	} else if _, err := os.Stat(target); os.IsNotExist(err) {
		draft, err := Discover(root)
		if err != nil {
			return false, fmt.Errorf("workspace retained at %s; cannot draft architecture: %w", root, err)
		}
		if err = os.WriteFile(target, draft, 0600); err != nil {
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
	scope, err := os.OpenRoot(root)
	if err != nil {
		return "", &application.SourceError{Kind: "missing", Message: "source unavailable"}
	}
	defer func() { _ = scope.Close() }()
	source, err := scope.Open(filepath.FromSlash(file))
	if err != nil {
		kind := "forbidden"
		if os.IsNotExist(err) {
			kind = "missing"
		}
		return "", &application.SourceError{Kind: kind, Message: "source unavailable or outside project"}
	}
	defer func() { _ = source.Close() }()
	info, err := source.Stat()
	const maxSource = 2 * 1024 * 1024
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxSource {
		return "", &application.SourceError{Kind: "large", Message: "source unavailable or too large"}
	}
	b, err := io.ReadAll(io.LimitReader(source, maxSource+1))
	if err != nil {
		return "", &application.SourceError{Kind: "missing", Message: "source unavailable"}
	}
	if len(b) > maxSource {
		return "", &application.SourceError{Kind: "large", Message: "source too large"}
	}
	var numbered strings.Builder
	for i, line := range strings.Split(string(b), "\n") {
		fmt.Fprintf(&numbered, "%5d  %s\n", i+1, line)
	}
	return numbered.String(), nil
}

func ConfigKey(root string) (string, error) {
	// #nosec G304 -- The cache key reads only the selected project's constant configuration filename.
	b, err := os.ReadFile(filepath.Join(root, config.Filename))
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}
