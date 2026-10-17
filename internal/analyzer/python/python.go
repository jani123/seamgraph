package python

import (
	"context"
	_ "embed"
	"seamgraph/internal/analyzer"
	"seamgraph/internal/config"
	"seamgraph/internal/ir"
)

//go:embed helper.py
var helper []byte

type Adapter struct{}

func (Adapter) Analyze(ctx context.Context, p analyzer.Project, c config.Config) (ir.PartialSnapshot, error) {
	b, err := analyzer.NewBuilder(p.Root, c, "python")
	if err != nil {
		return ir.PartialSnapshot{}, err
	}
	return analyzer.RunHelper(ctx, b.Root, c, "python3", "helper.py", map[string][]byte{"helper.py": helper}, "-I")
}
