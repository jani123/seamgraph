// Package registry is the adapter dispatch boundary. Downstream consumers receive
// only validated IR and never need to know which source languages produced it.
package registry

import (
	"context"
	"fmt"
	"path/filepath"

	"seamgraph/internal/analyzer"
	"seamgraph/internal/analyzer/golang"
	"seamgraph/internal/analyzer/python"
	"seamgraph/internal/analyzer/typescript"
	"seamgraph/internal/config"
	"seamgraph/internal/ir"
)

func Scan(ctx context.Context, root string) (ir.Snapshot, config.Config, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return ir.Snapshot{}, config.Config{}, err
	}
	doc, err := config.Load(root)
	if err != nil {
		return ir.Snapshot{}, config.Config{}, err
	}
	adapters := map[string]analyzer.Adapter{"go": golang.Adapter{}, "typescript": typescript.Adapter{}, "python": python.Adapter{}}
	parts := []ir.PartialSnapshot{}
	for _, language := range []string{"go", "typescript", "python"} {
		selected := false
		for _, m := range doc.Config.Modules {
			if m.Language == language {
				selected = true
			}
		}
		if !selected {
			continue
		}
		part, err := adapters[language].Analyze(ctx, analyzer.Project{Root: root}, doc.Config)
		if err != nil {
			return ir.Snapshot{}, doc.Config, fmt.Errorf("%s: %w", language, err)
		}
		parts = append(parts, part)
	}
	snapshot, err := ir.Merge(parts...)
	return snapshot, doc.Config, err
}
