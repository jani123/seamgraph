package projectfiles

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"seamgraph/internal/verification"
)

func evidencePath(root string) string { return filepath.Join(root, ".seamgraph", "evidence.json") }
func ReadStore(root string) (verification.Store, error) {
	s := verification.Store{SchemaVersion: 1, Evidence: []verification.Evidence{}}
	dirInfo, err := os.Lstat(filepath.Dir(evidencePath(root)))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if !dirInfo.IsDir() {
		return s, fmt.Errorf("evidence directory must not be a symlink")
	}
	info, err := os.Lstat(evidencePath(root))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if !info.Mode().IsRegular() {
		return s, fmt.Errorf("evidence store must be a regular file")
	}
	b, err := os.ReadFile(evidencePath(root))
	if err != nil {
		return s, err
	}
	s = verification.Store{}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&s); err != nil {
		return s, err
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return s, fmt.Errorf("trailing evidence store data")
	}
	if s.SchemaVersion != 1 {
		return s, fmt.Errorf("unsupported evidence store version")
	}
	return s, nil
}
func EvidenceDir(root string) (string, error) {
	dir := filepath.Join(root, ".seamgraph")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf(".seamgraph must be a real project directory")
	}
	return dir, nil
}
func AppendEvidence(root string, evidence []verification.Evidence) (verification.Store, error) {
	var store verification.Store
	dir, err := EvidenceDir(root)
	if err != nil {
		return store, err
	}
	lock := filepath.Join(dir, "evidence.lock")
	f, err := os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return store, fmt.Errorf("evidence store is locked or unavailable (%s): %w", lock, err)
	}
	f.Close()
	defer os.Remove(lock)
	store, err = ReadStore(root)
	if err != nil {
		return store, err
	}
	known := map[string]string{}
	for _, e := range store.Evidence {
		b, _ := json.Marshal(e)
		known[e.ID] = string(b)
	}
	for _, e := range evidence {
		b, _ := json.Marshal(e)
		if old, ok := known[e.ID]; ok {
			if old != string(b) {
				return store, fmt.Errorf("conflicting evidence identity %s", e.ID)
			}
			continue
		}
		store.Evidence = append(store.Evidence, e)
		known[e.ID] = string(b)
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return store, err
	}
	temp, err := os.CreateTemp(dir, "evidence-*.tmp")
	if err != nil {
		return store, err
	}
	defer os.Remove(temp.Name())
	if _, err = temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		return store, err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return store, err
	}
	if err = temp.Close(); err != nil {
		return store, err
	}
	err = os.Rename(temp.Name(), evidencePath(root))
	return store, err
}
