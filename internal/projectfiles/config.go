package projectfiles

import (
	"os"
	"path/filepath"
	"seamgraph/internal/config"
)

func Load(root string) (*config.Document, error) {
	b, err := os.ReadFile(filepath.Join(root, config.Filename))
	if err != nil {
		return nil, err
	}
	return config.Parse(b)
}
