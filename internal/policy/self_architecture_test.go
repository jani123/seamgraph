package policy_test

import (
	"seamgraph/internal/application"
	"seamgraph/internal/projectfiles"
	"strings"
	"testing"

	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
)

// These cases lock intended architectural constraints independently of today's
// imports. A clean scan alone would not catch a future broadening of the rules.
func TestSelfArchitectureBoundaries(t *testing.T) {
	doc, err := projectfiles.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	p, err := application.NormalizePolicy(doc.Config)
	if err != nil {
		t.Fatal(err)
	}
	if p.Default != "deny" || p.Unresolved != "error" {
		t.Fatal("self-policy must fail closed")
	}
	for _, rule := range p.Rules {
		if rule.Effect == "allow" && !strings.HasPrefix(rule.ToID, "seam:") {
			t.Fatalf("module-wide grant weakens boundary: %+v", rule)
		}
	}
	for _, tc := range []struct {
		from, to, selector string
		allowed            bool
	}{
		{"cli", "application.inspection", "internal/application.Service.InspectProject", true},
		{"cli", "application.evidence", "internal/application.Service.RunTests", true},
		{"web", "application.explorer", "internal/application.Service.Load", true},
		{"web", "application.rules", "internal/application.Service.Confirm", true},
		{"web", "application.evidence", "internal/application.Service.RunTests", false},
		{"cli", "application.rules", "internal/application.Service.Confirm", false},
		{"ui", "application.views", "internal/application.Model", true},
		{"ui", "application.explorer", "internal/application.Service.Load", false},
		{"ui", "application.rules", "internal/application.Service.Confirm", false},
		{"cli", "project-files.access", "internal/projectfiles.PrepareRule", false},
		{"web", "project-files.access", "internal/projectfiles.PrepareRule", false},
		{"application", "project-files.access", "internal/projectfiles.ReadStore", false},
		{"application", "git-workspaces.access", "internal/gitworkspace.Extract", false},
		{"bootstrap", "git-workspaces.access", "internal/gitworkspace.Extract", true},
		{"bootstrap", "go-adapter.construction", "internal/analyzer/golang.Adapter", true},
		{"analysis", "go-adapter.construction", "internal/analyzer/golang.Adapter", false},
		{"go-adapter", "python-adapter.construction", "internal/analyzer/python.Adapter", false},
		{"python-adapter", "typescript-adapter.construction", "internal/analyzer/typescript.Adapter", false},
		{"typescript-adapter", "go-adapter.construction", "internal/analyzer/golang.Adapter", false},
		{"policy", "application.explorer", "internal/application.Service.Load", false},
		{"comparison", "git-workspaces.access", "internal/gitworkspace.Extract", false},
		{"git-workspaces", "comparison.architecture", "internal/snapshot.ComparePolicies", false},
		{"project", "policy.evaluation", "internal/policy.Evaluate", false},
		{"ui", "policy.evaluation", "internal/policy.Evaluate", false},
		{"application", "policy.evaluation", "internal/policy.Evaluate", true},
	} {
		t.Run(tc.from+"->"+tc.to, func(t *testing.T) {
			modules, seams := doc.Config.IRModules()
			targetModule, _, _ := strings.Cut(tc.to, ".")
			symbol := ir.Symbol{ID: ir.SymbolID(targetModule, tc.selector), ModuleID: ir.ModuleID(targetModule), Kind: "function", DisplaySignature: tc.selector, Visibility: "public"}
			for i := range seams {
				if seams[i].ID == "seam:"+tc.to {
					seams[i].ContractMembers = []string{symbol.ID}
				}
			}
			from := ir.ModuleID(tc.from)
			edge := ir.Relation{ID: ir.RelationID("uses", from, symbol.ID), FromID: from, ToID: symbol.ID, Kind: "uses", Evidence: []ir.Evidence{{File: "architecture-probe.go", StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 2, Confidence: "exact", Resolution: "resolved"}}}
			s := ir.Snapshot{SchemaVersion: 1, Modules: modules, Seams: seams, Symbols: []ir.Symbol{symbol}, Relations: []ir.Relation{edge}, Diagnostics: []ir.Diagnostic{}}
			result, err := policy.Evaluate(s, p)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(result.Violations) == 0; got != tc.allowed {
				t.Fatalf("allowed=%t, want %t: %+v", got, tc.allowed, result.Violations)
			}
		})
	}
}
