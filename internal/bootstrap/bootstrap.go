// Package bootstrap is the only concrete implementation composition root.
package bootstrap

import (
	"context"
	"os"
	"runtime"
	"seamgraph/internal/analysis"
	"seamgraph/internal/analyzer/golang"
	"seamgraph/internal/analyzer/python"
	"seamgraph/internal/analyzer/testresults"
	"seamgraph/internal/analyzer/typescript"
	"seamgraph/internal/application"
	"seamgraph/internal/gitworkspace"
	"seamgraph/internal/projectfiles"
	"seamgraph/internal/verification"
	"strings"
)

func scanner() analysis.Scanner {
	return analysis.Scanner{Load: projectfiles.Load, Adapters: map[string]analysis.Adapter{"go": golang.Adapter{}, "typescript": typescript.Adapter{}, "python": python.Adapter{}}}
}
func New(root string) (*application.Service, error) {
	return application.New(root, application.Ports{
		ConfigKey: projectfiles.ConfigKey, Inventory: projectfiles.Inventory,
		Index: scanner().Index, BrowseInventory: projectfiles.BrowseInventory,
		Activity: gitworkspace.Activity, ReadReview: projectfiles.ReadReview, AppendReview: projectfiles.AppendReview,
		Scan: scanner().Scan, LoadConfig: projectfiles.Load, LoadContracts: projectfiles.LoadContracts,
		ReadStore: projectfiles.ReadStore, AppendEvidence: projectfiles.AppendEvidence, EvidenceDir: projectfiles.EvidenceDir,
		ReadFile: os.ReadFile, Execute: testresults.Execute,
		Import: func(format string, b []byte, run application.EvidenceRun) ([]verification.Evidence, error) {
			return testresults.Import(format, b, testresults.Run(run))
		},
		Identity: gitworkspace.Identity, Extract: gitworkspace.Extract,
		Open: func(ctx context.Context, url, ref, cache string) (application.Workspace, error) {
			w, err := gitworkspace.Open(ctx, url, ref, cache)
			return application.Workspace(w), err
		},
		PrepareProject: projectfiles.PrepareProject, Initialize: projectfiles.Initialize, SetupAgent: projectfiles.SetupAgent,
		PrepareRule: func(root, effect, from, to string) (application.RuleEdit, error) {
			return projectfiles.PrepareRule(root, effect, from, to)
		},
		Source: projectfiles.Source,
	}, runtime.Version()+"|"+strings.Join(os.Environ(), "\x00"))
}
