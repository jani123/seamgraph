package typescript

import (
	"context"
	"embed"
	"io/fs"
	"path/filepath"

	"seamgraph/internal/analyzer"
	"seamgraph/internal/config"
	"seamgraph/internal/ir"
)

//go:embed helper.cjs lib*.d.ts TYPESCRIPT_*
var assets embed.FS

type Adapter struct{}

func (Adapter) Analyze(ctx context.Context, p analyzer.Project, c config.Config) (ir.PartialSnapshot, error) {
	b, err := analyzer.NewBuilder(p.Root, c, "typescript")
	if err != nil {
		return ir.PartialSnapshot{}, err
	}
	files := map[string][]byte{}
	err = fs.WalkDir(assets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			data, err := assets.ReadFile(path)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(path)] = data
		}
		return nil
	})
	if err != nil {
		return ir.PartialSnapshot{}, err
	}
	return analyzer.RunHelper(ctx, b.Root, c, "node", "helper.cjs", files)
}
