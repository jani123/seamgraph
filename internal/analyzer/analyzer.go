// Package analyzer declares the common adapter contract. Implementations belong
// in child packages; downstream packages must consume only normalized IR.
package analyzer

import (
	"context"
	"seamgraph/internal/config"
	"seamgraph/internal/ir"
)

type Project struct{ Root string }
type Adapter interface {
	Analyze(context.Context, Project, config.Config) (ir.PartialSnapshot, error)
}
