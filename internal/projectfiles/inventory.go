package projectfiles

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Inventory fingerprints materialized comparison trees, never the live checkout.
func Inventory(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported comparison file: %s", file)
		}
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		hash := sha256.New()
		fmt.Fprintf(hash, "%d\x00", info.Mode().Perm()&0111)
		_, readErr := io.Copy(hash, f)
		closeErr := f.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = fmt.Sprintf("%x", hash.Sum(nil))
		return nil
	})
	return files, err
}
