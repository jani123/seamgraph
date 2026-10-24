package projectfiles

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// BrowseInventory enumerates regular project files without Git or architecture
// configuration. Build/dependency caches and symlinks are excluded explicitly.
func BrowseInventory(root string) (map[string]string, error) {
	scope, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer func() { _ = scope.Close() }()
	files := map[string]string{}
	ignored := map[string]bool{".git": true, ".seamgraph": true, ".build": true, ".devbox": true, ".codex-local": true, ".venv": true, "__pycache__": true, "node_modules": true, "vendor": true, "dist": true, "build": true, ".pytest_cache": true}
	err = filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if file != root && ignored[entry.Name()] {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		name, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if len(files) >= 20000 {
			return fmt.Errorf("directory exceeds 20000 files; select a smaller directory")
		}
		source, err := scope.Open(name)
		if err != nil {
			return err
		}
		digest := sha256.New()
		_, copyErr := io.Copy(digest, source)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		files[filepath.ToSlash(name)] = fmt.Sprintf("%x", digest.Sum(nil))
		return nil
	})
	return files, err
}
