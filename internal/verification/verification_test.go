package verification_test

import (
	"os"
	"testing"

	"seamgraph/internal/analyzer/testresults"
	"seamgraph/internal/ir"
	"seamgraph/internal/verification"
)

func model(t *testing.T) ir.Snapshot {
	t.Helper()
	b, err := os.ReadFile("../../testdata/ir/go.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := ir.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func manifest() verification.Manifest {
	return verification.Manifest{Version: 1, Requirements: []verification.Requirement{{ID: "behavior:read", SeamID: "seam:world.reader", MemberID: "symbol:world:api%2FRead", Description: "Read returns the requested value", Kind: "behavior"}}, Links: []verification.Link{{TestID: "Read returns value", RequirementIDs: []string{"behavior:read"}, MemberIDs: []string{"symbol:world:api%2FRead"}, Reviewed: true}}, Policies: map[string]string{"seam:world.reader": "require-verified"}}
}
func TestEquivalentRunnerEvidence(t *testing.T) {
	for _, tc := range []struct{ format, data, id string }{{"go-test-json", "{\"Action\":\"pass\",\"Package\":\"example\",\"Test\":\"TestRead\"}\n", "example/TestRead"}, {"vitest-json", `{"testResults":[{"assertionResults":[{"fullName":"Read returns value","status":"passed"}]}]}`, "Read returns value"}, {"junit", `<testsuites><testsuite><testcase classname="test_api" name="test_read"/></testsuite></testsuites>`, "test_api/test_read"}} {
		t.Run(tc.format, func(t *testing.T) {
			m := manifest()
			m.Links[0].TestID = tc.id
			e, err := testresults.Import(tc.format, []byte(tc.data), testresults.Run{Runner: tc.format, Revision: "abc", Fingerprint: "inputs", RunID: "run", ObservedAt: "2026-10-16T00:00:00Z"})
			if err != nil {
				t.Fatal(err)
			}
			report, err := verification.Evaluate(model(t), m, verification.Store{SchemaVersion: 1, Evidence: e}, "abc", "inputs")
			if err != nil || report.ExitCode != 0 || report.Results[0].Status != "verified" || report.Results[0].RequirementCoverage.Covered != 1 {
				t.Fatal(report, err)
			}
		})
	}
}
func TestNoFalseVerification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*verification.Manifest, *verification.Store)
		want   string
		code   int
	}{
		{"missing artifact", func(m *verification.Manifest, s *verification.Store) { s.Evidence = nil }, "unknown", 2},
		{"missing mapping", func(m *verification.Manifest, s *verification.Store) { m.Links = nil }, "unverified", 1},
		{"execution without reviewed assertions", func(m *verification.Manifest, s *verification.Store) { m.Links[0].Reviewed = false }, "unverified", 1},
		{"95 percent line coverage", func(m *verification.Manifest, s *verification.Store) { s.Evidence[0].EvidenceKind = "coverage" }, "unknown", 2},
		{"failed requirement", func(m *verification.Manifest, s *verification.Store) { s.Evidence[0].Outcome = "failed" }, "failed", 1},
		{"changed implementation", func(m *verification.Manifest, s *verification.Store) { s.Evidence[0].Fingerprint = "old" }, "stale", 2},
		{"changed revision", func(m *verification.Manifest, s *verification.Store) { s.Evidence[0].Revision = "old" }, "stale", 2},
		{"skipped", func(m *verification.Manifest, s *verification.Store) { s.Evidence[0].Outcome = "skipped" }, "unknown", 2},
		{"undeclared public behavior", func(m *verification.Manifest, s *verification.Store) {
			m.Requirements = append(m.Requirements, verification.Requirement{ID: "behavior:boundary", SeamID: "seam:world.reader", Description: "Boundary error is observable", Kind: "boundary"})
		}, "partial", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest()
			s := verification.Store{SchemaVersion: 1, Evidence: []verification.Evidence{{ID: "e:run", TestID: m.Links[0].TestID, Runner: "test", Revision: "abc", Fingerprint: "inputs", RunID: "run", ObservedAt: "2026-10-16T00:00:00Z", Outcome: "passed", EvidenceKind: "test-execution"}}}
			tc.change(&m, &s)
			r, err := verification.Evaluate(model(t), m, s, "abc", "inputs")
			if err != nil || r.Results[0].Status != tc.want || r.ExitCode != tc.code {
				t.Fatal(r, err)
			}
		})
	}
}
func TestRerunReplacesHistoricalFailure(t *testing.T) {
	m := manifest()
	s := verification.Store{SchemaVersion: 1, Evidence: []verification.Evidence{{ID: "e:old", TestID: m.Links[0].TestID, Runner: "test", Revision: "abc", Fingerprint: "old", RunID: "old", ObservedAt: "2026-10-15T00:00:00Z", Outcome: "failed", EvidenceKind: "test-execution"}, {ID: "e:new", TestID: m.Links[0].TestID, Runner: "test", Revision: "abc", Fingerprint: "inputs", RunID: "new", ObservedAt: "2026-10-16T00:00:00Z", Outcome: "passed", EvidenceKind: "test-execution"}}}
	r, err := verification.Evaluate(model(t), m, s, "abc", "inputs")
	if err != nil || r.Results[0].Status != "verified" {
		t.Fatal(r, err)
	}
}

func TestPartialCoverageDoesNotInventCompleteMeasurement(t *testing.T) {
	s := model(t)
	write := ir.Symbol{ID: ir.SymbolID("world", "api/Write"), ModuleID: "module:world", Kind: "function", DisplaySignature: "Write(Value)", Visibility: "public"}
	s.Symbols = append(s.Symbols, write)
	s.Seams[0].ContractMembers = append(s.Seams[0].ContractMembers, write.ID)
	m := manifest()
	store := verification.Store{SchemaVersion: 1, Evidence: []verification.Evidence{{ID: "e:coverage", TestID: m.Links[0].TestID, Runner: "test", Revision: "abc", Fingerprint: "inputs", RunID: "run", ObservedAt: "2026-10-16T00:00:00Z", Outcome: "passed", EvidenceKind: "coverage", MemberIDs: []string{"symbol:world:api%2FRead"}}}}
	r, err := verification.Evaluate(s, m, store, "abc", "inputs")
	if err != nil {
		t.Fatal(err)
	}
	c := r.Results[0].MemberCoverage
	if c.Known || c.Covered != 1 || c.Total != 2 || r.Results[0].Status == "verified" {
		t.Fatal("partial line/member evidence became complete verification", r)
	}
}

func TestEnforcingModesCannotPassUnknownExecution(t *testing.T) {
	for _, mode := range []string{"report", "fail-on-failed", "fail-on-missing", "require-verified"} {
		m := manifest()
		m.Policies["seam:world.reader"] = mode
		r, err := verification.Evaluate(model(t), m, verification.Store{SchemaVersion: 1}, "abc", "inputs")
		if err != nil {
			t.Fatal(err)
		}
		want := 2
		if mode == "report" {
			want = 0
		}
		if r.ExitCode != want || r.Results[0].Status != "unknown" {
			t.Fatal("incomplete execution evidence silently passed enforcing mode", mode, r)
		}
	}
}

func TestConflictingSameTimeEvidenceCannotChoosePassByID(t *testing.T) {
	m := manifest()
	passed := verification.Evidence{ID: "e:z-passed", TestID: m.Links[0].TestID, Runner: "test", Revision: "abc", Fingerprint: "inputs", RunID: "passed", ObservedAt: "2026-10-16T00:00:00Z", Outcome: "passed", EvidenceKind: "test-execution"}
	failed := passed
	failed.ID = "e:a-failed"
	failed.RunID = "failed"
	failed.Outcome = "failed"
	for _, evidence := range [][]verification.Evidence{{passed, failed}, {failed, passed}} {
		r, err := verification.Evaluate(model(t), m, verification.Store{SchemaVersion: 1, Evidence: evidence}, "abc", "inputs")
		if err != nil || r.Results[0].Status != "failed" || r.ExitCode != 1 {
			t.Fatal("ambiguous timestamp hid failure behind arbitrary ID order", err, r)
		}
	}
}

func TestCurrentIdentityWinsOverLaterUnrelatedRun(t *testing.T) {
	m := manifest()
	current := verification.Evidence{ID: "e:current", TestID: m.Links[0].TestID, Runner: "test", Revision: "abc", Fingerprint: "inputs", RunID: "current", ObservedAt: "2026-10-16T00:00:00Z", Outcome: "passed", EvidenceKind: "test-execution"}
	unrelated := current
	unrelated.ID, unrelated.RunID = "e:later", "later"
	unrelated.Fingerprint = "different-checkout"
	unrelated.ObservedAt, unrelated.Outcome = "2026-10-17T00:00:00Z", "failed"
	coverage := current
	coverage.ID, coverage.EvidenceKind = "e:coverage", "coverage"
	coverage.MemberIDs = m.Links[0].MemberIDs
	for _, evidence := range [][]verification.Evidence{{current, unrelated, coverage}, {coverage, unrelated, current}} {
		r, err := verification.Evaluate(model(t), m, verification.Store{SchemaVersion: 1, Evidence: evidence}, "abc", "inputs")
		if err != nil {
			t.Fatal(err)
		}
		if r.ExitCode != 0 || r.Results[0].Status != "verified" || !r.Results[0].MemberCoverage.Known || len(r.Results[0].StaleEvidence) != 0 {
			t.Fatal("unrelated execution or coverage superseded current assertion evidence", r)
		}
	}
}

func TestIncompleteSeamKeepsPrecedenceOverLaterViolation(t *testing.T) {
	s := model(t)
	s.Seams = append(s.Seams, ir.Seam{ID: "seam:world.writer", ModuleID: "module:world", ContractMembers: []string{}})
	for i := range s.Modules {
		if s.Modules[i].ID == "module:world" {
			s.Modules[i].Seams = append(s.Modules[i].Seams, "seam:world.writer")
		}
	}
	m := manifest()
	m.Policies["seam:world.writer"] = "require-verified"
	m.Requirements = append(m.Requirements, verification.Requirement{ID: "behavior:write", SeamID: "seam:world.writer", Description: "Write preserves its input", Kind: "behavior"})
	r, err := verification.Evaluate(s, m, verification.Store{SchemaVersion: 1}, "abc", "inputs")
	if err != nil {
		t.Fatal(err)
	}
	if r.ExitCode != 2 || len(r.Results) != 2 || r.Results[0].Status != "unknown" || r.Results[1].Status != "unverified" {
		t.Fatal("later missing requirement hid incomplete evidence from an earlier seam", r)
	}
}
