package snapshot

import (
	"os"
	"testing"

	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"seamgraph/internal/verification"
)

func load(t *testing.T, name string) ir.Snapshot {
	t.Helper()
	b, err := os.ReadFile("../../testdata/ir/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := ir.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestSharedDiffContract(t *testing.T) {
	expected := map[string]string{}
	for _, name := range []string{"go", "typescript", "python"} {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name         string
				mutate       func(*ir.Snapshot)
				kind, status string
				count        int
			}{
				{name: "unchanged", mutate: func(s *ir.Snapshot) {}},
				{name: "location only", mutate: func(s *ir.Snapshot) {
					s.Relations[0].Evidence[0].StartLine = 20
					s.Relations[0].Evidence[0].EndLine = 20
				}},
				{name: "signature", mutate: func(s *ir.Snapshot) { s.Symbols[0].DisplaySignature = "Read(id string) Value" }, kind: "symbol", status: "modified", count: 1},
				{name: "removed consumer", mutate: func(s *ir.Snapshot) { s.Relations = nil }, kind: "relation", status: "removed", count: 1},
				{name: "new module", mutate: func(s *ir.Snapshot) {
					s.Modules = append(s.Modules, ir.Module{ID: "module:extra", DisplayName: "extra", SourceRoots: []string{"src/extra"}})
				}, kind: "module", status: "added", count: 1},
				{name: "unconsumed exported internal edit", mutate: func(s *ir.Snapshot) {
					s.Symbols = append(s.Symbols, ir.Symbol{ID: ir.SymbolID("world", "internal/Helper"), ModuleID: "module:world", Kind: "function", DisplaySignature: "Helper()", Visibility: "public"})
				}},
				{name: "private internal edit", mutate: func(s *ir.Snapshot) {
					s.Symbols = append(s.Symbols, ir.Symbol{ID: ir.SymbolID("world", "private/Helper"), ModuleID: "module:world", Kind: "function", DisplaySignature: "Helper()", Visibility: "private"})
					r := s.Relations[0]
					r.FromID = "module:world"
					r.ToID = s.Symbols[1].ID
					r.ID = ir.RelationID(r.Kind, r.FromID, r.ToID)
					s.Relations = append(s.Relations, r)
				}},
				{name: "deep bypass", mutate: func(s *ir.Snapshot) { s.Relations[0].BypassesSeam = true }, kind: "relation", status: "modified", count: 1},
				{name: "seam rename", mutate: func(s *ir.Snapshot) {
					s.Seams[0].ID = "seam:world.renamed"
					s.Modules[1].Seams = []string{s.Seams[0].ID}
				}, count: 3},
			} {
				t.Run(tc.name, func(t *testing.T) {
					base, head := load(t, name), load(t, name)
					tc.mutate(&head)
					d, err := Compare(base, head)
					if err != nil {
						t.Fatal(err)
					}
					if len(d.Changes) != tc.count {
						t.Fatalf("changes %+v; want %d", d, tc.count)
					}
					if tc.kind != "" && (d.Changes[0].Kind != tc.kind || d.Changes[0].Status != tc.status) {
						t.Fatal("wrong change", d)
					}
					b, err := d.JSON()
					if err != nil {
						t.Fatal(err)
					}
					if want, ok := expected[tc.name]; ok && string(b) != want {
						t.Fatalf("language-origin fixture changed architecture diff: %s", b)
					}
					expected[tc.name] = string(b)
				})
			}
		})
	}
}
func TestSharedPolicyRegressions(t *testing.T) {
	var expected string
	for _, name := range []string{"go", "typescript", "python"} {
		t.Run(name, func(t *testing.T) {
			base, head := load(t, name), load(t, name)
			base.Relations = nil
			p := policy.Policy{Default: "deny", Unresolved: "error"}
			d, err := ComparePolicies(base, head, p, p)
			if err != nil {
				t.Fatal(err)
			}
			if len(d.Changes) != 2 {
				t.Fatal("expected new edge and violation", d)
			}
			b, _ := d.JSON()
			if expected != "" && string(b) != expected {
				t.Fatal("language-origin changed regression")
			}
			expected = string(b)
			reverse, err := ComparePolicies(head, base, p, p)
			if err != nil || len(reverse.Changes) != 2 {
				t.Fatal(reverse, err)
			}
			for _, c := range reverse.Changes {
				if c.Status != "removed" {
					t.Fatal("regression not resolved", c)
				}
			}
		})
	}
}
func TestInvalidAndIncompleteInput(t *testing.T) {
	s := load(t, "go")
	bad := s
	bad.SchemaVersion = 999
	if _, err := Compare(bad, s); err == nil {
		t.Fatal("invalid base accepted")
	}
	if _, err := Compare(s, bad); err == nil {
		t.Fatal("invalid head accepted")
	}
	s.Diagnostics = []ir.Diagnostic{{Code: "LOAD_FAILED", Severity: "error", Message: "package loading failed"}}
	p := policy.Policy{Default: "allow", Unresolved: "error"}
	if _, err := ComparePolicies(s, s, p, p); err == nil {
		t.Fatal("incomplete analysis reported no regression")
	}
}

func TestVerificationRegressionsAndRequirementEdits(t *testing.T) {
	base := verification.Report{SchemaVersion: 1, Requirements: []verification.Requirement{{ID: "behavior:read", SeamID: "seam:world.reader", Description: "Read returns value", Kind: "behavior"}}, Results: []verification.Result{{SeamID: "seam:world.reader", Revision: "base", Status: "verified", RequirementCoverage: verification.Coverage{Covered: 1, Total: 1}}}}
	head := base
	head.Results = []verification.Result{{SeamID: "seam:world.reader", Revision: "head", Status: "stale", RequirementCoverage: verification.Coverage{Total: 1}}}
	changes := CompareVerification(base, head)
	if len(changes) != 1 || changes[0].Kind != "verification" || changes[0].Status != "modified" {
		t.Fatal(changes)
	}
	head = base
	head.Results = append([]verification.Result{}, base.Results...)
	head.Results[0].Revision = "other"
	if got := CompareVerification(base, head); len(got) != 0 {
		t.Fatal("revision-only metadata polluted contract diff", got)
	}
	head.Requirements = append([]verification.Requirement{}, base.Requirements...)
	head.Requirements[0].Description = "Read rejects unknown identifiers"
	if got := CompareVerification(base, head); len(got) != 1 || got[0].Kind != "requirement" {
		t.Fatal("requirement change missed", got)
	}
}
