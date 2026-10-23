package projectfiles

import (
	"os"
	"path/filepath"
	"seamgraph/internal/config"
)

func Load(root string) (*config.Document, error) {
	// #nosec G304 -- Root is the selected project directory and the configuration filename is constant.
	b, err := os.ReadFile(filepath.Join(root, config.Filename))
	if err != nil {
		return nil, err
	}
	return config.Parse(b)
}
