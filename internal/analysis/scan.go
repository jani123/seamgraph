// Package analysis is the adapter dispatch boundary. Downstream consumers receive
// only validated IR and never need to know which source languages produced it.
package analysis

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"seamgraph/internal/config"
	"seamgraph/internal/ir"
)

type Scanner struct {
	Adapters map[string]Adapter
	Load     func(string) (*config.Document, error)
}

func (s Scanner) Scan(ctx context.Context, root string) (ir.Snapshot, config.Config, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return ir.Snapshot{}, config.Config{}, err
	}
	doc, err := s.Load(root)
	if err != nil {
		return ir.Snapshot{}, config.Config{}, err
	}
	parts := []ir.PartialSnapshot{}
	languages := map[string]bool{}
	for _, m := range doc.Config.Modules {
		languages[m.Language] = true
	}
	selected := make([]string, 0, len(languages))
	for language := range languages {
		selected = append(selected, language)
	}
	sort.Strings(selected)
	for _, language := range selected {
		if s.Adapters[language] == nil {
			return ir.Snapshot{}, doc.Config, fmt.Errorf("no adapter registered for %s", language)
		}
		part, err := s.Adapters[language].Analyze(ctx, Project{Root: root}, doc.Config)
		if err != nil {
			return ir.Snapshot{}, doc.Config, fmt.Errorf("%s: %w", language, err)
		}
		parts = append(parts, part)
	}
	snapshot, err := ir.Merge(parts...)
	return snapshot, doc.Config, err
}
