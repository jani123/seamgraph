package projectfiles

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"seamgraph/internal/application"
)

// ReadReview reads only the repository-local append-only review log. OpenRoot
// prevents a substituted .seamgraph symlink from escaping the selected project.
func ReadReview(root string) ([]application.ReviewEntry, error) {
	entries := []application.ReviewEntry{}
	scope, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer func() { _ = scope.Close() }()
	for _, name := range []string{".seamgraph", ".seamgraph/reviews.jsonl"} {
		info, err := scope.Lstat(name)
		if os.IsNotExist(err) {
			return entries, nil
		}
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || (name == ".seamgraph/reviews.jsonl" && !info.Mode().IsRegular()) {
			return nil, fmt.Errorf("review state must use real directories and regular files")
		}
	}
	file, err := scope.Open(".seamgraph/reviews.jsonl")
	if os.IsNotExist(err) {
		return entries, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 16*1024*1024 {
		return nil, fmt.Errorf("review log is not a bounded regular file")
	}
	scanner := bufio.NewScanner(io.LimitReader(file, 16*1024*1024+1))
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for scanner.Scan() {
		var entry application.ReviewEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("invalid review log: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

// AppendReview serializes writers and preserves previous anchors and notes.
func AppendReview(root string, entry application.ReviewEntry) error {
	if _, err := EvidenceDir(root); err != nil {
		return err
	}
	scope, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { _ = scope.Close() }()
	lock, err := scope.OpenFile(".seamgraph/reviews.lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("review log locked or unavailable: %w", err)
	}
	_ = lock.Close()
	defer func() { _ = scope.Remove(".seamgraph/reviews.lock") }()
	if _, err := ReadReview(root); err != nil {
		return err
	}
	file, err := scope.OpenFile(".seamgraph/reviews.jsonl", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size()+int64(len(data))+1 > 16*1024*1024 {
		return fmt.Errorf("review log reached its 16 MiB limit; archive it before saving more notes")
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return file.Sync()
}
