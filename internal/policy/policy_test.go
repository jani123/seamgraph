package policy

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"seamgraph/internal/ir"
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
func grant(to, effect string) Rule { return Rule{FromID: "module:npc", ToID: to, Effect: effect} }
func TestSharedPolicyContract(t *testing.T) {
	var equivalent []byte
	for _, name := range []string{"go", "typescript", "python"} {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name              string
				def               string
				rules             []Rule
				bypass, nonmember bool
				want              int
				code              string
			}{
				{name: "default deny", def: "deny", want: 1, code: "SEAM_USE_DENIED"},
				{name: "seam grant", def: "deny", rules: []Rule{grant("seam:world.reader", "allow")}},
				{name: "module grant is not member permission", def: "deny", rules: []Rule{grant("module:world", "allow")}, want: 1, code: "SEAM_USE_DENIED"},
				{name: "deny wins", def: "deny", rules: []Rule{grant("module:world", "deny"), grant("seam:world.reader", "allow")}, want: 1, code: "SEAM_USE_DENIED"},
				{name: "default allow", def: "allow"},
				{name: "explicit seam deny", def: "allow", rules: []Rule{grant("seam:world.reader", "deny")}, want: 1, code: "SEAM_USE_DENIED"},
				{name: "module grant cannot allow bypass", def: "deny", rules: []Rule{grant("module:world", "allow")}, bypass: true, want: 1, code: "SEAM_BYPASS"},
				{name: "deep import bypass", def: "allow", rules: []Rule{grant("seam:world.reader", "allow")}, bypass: true, want: 1, code: "SEAM_BYPASS"},
				{name: "nonmember bypass", def: "allow", nonmember: true, want: 1, code: "SEAM_BYPASS"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					s := load(t, name)
					s.Relations[0].BypassesSeam = tc.bypass
					if tc.nonmember {
						s.Seams[0].ContractMembers = nil
					}
					p := Policy{Default: tc.def, Unresolved: "error", Rules: tc.rules}
					r, err := Evaluate(s, p)
					if err != nil {
						t.Fatal(err)
					}
					if r.ExitCode() != tc.want {
						t.Fatalf("exit %d; want %d: %+v", r.ExitCode(), tc.want, r)
					}
					if tc.code != "" && (len(r.Violations) != 1 || r.Violations[0].Code != tc.code) {
						t.Fatalf("wrong violation %+v", r)
					}
					if tc.name == "default deny" {
						for i := range r.Violations {
							r.Violations[i].Evidence = []ir.Evidence{}
						}
						b, _ := json.Marshal(r)
						if equivalent == nil {
							equivalent = b
						} else if string(equivalent) != string(b) {
							t.Fatal("language-origin fixture changed policy result")
						}
					}
				})
			}
		})
	}
}
func TestDependencyDoesNotProveSeamUsage(t *testing.T) {
	for _, name := range []string{"go", "typescript", "python"} {
		t.Run(name, func(t *testing.T) {
			s := load(t, name)
			r := s.Relations[0]
			r.Kind = "imports"
			r.ToID = "module:world"
			r.ID = ir.RelationID(r.Kind, r.FromID, r.ToID)
			s.Relations = []ir.Relation{r}
			for _, rules := range [][]Rule{{grant("seam:world.reader", "allow")}, {grant("module:world", "allow")}} {
				result, err := Evaluate(s, Policy{Default: "deny", Unresolved: "error", Rules: rules})
				if err != nil {
					t.Fatal(err)
				}
				if result.ExitCode() != 0 || len(result.Decisions) != 1 || result.Decisions[0].Category != "dependency" || len(result.Decisions[0].SeamIDs) != 0 {
					t.Fatalf("import misclassified: %+v", result)
				}
			}
		})
	}
}
func TestUncertaintyPolicy(t *testing.T) {
	for _, name := range []string{"go", "typescript", "python"} {
		t.Run(name, func(t *testing.T) {
			for _, resolution := range []string{"unresolved", "dynamic"} {
				s := load(t, name)
				e := s.Relations[0].Evidence[0]
				e.Resolution = resolution
				s.Diagnostics = []ir.Diagnostic{{Code: "REFERENCE_UNKNOWN", Severity: "warning", Message: "Reference could not be proven", Evidence: []ir.Evidence{e}}}
				for _, mode := range []string{"error", "warn"} {
					r, err := Evaluate(s, Policy{Default: "allow", Unresolved: mode})
					if err != nil {
						t.Fatal(err)
					}
					want := 0
					if mode == "error" {
						want = 2
					}
					if r.ExitCode() != want || len(r.Diagnostics) != 1 {
						t.Fatalf("%s: %+v", mode, r)
					}
				}
				s.Diagnostics[0].Severity = "error"
				r, err := Evaluate(s, Policy{Default: "allow", Unresolved: "warn"})
				if err != nil || r.ExitCode() != 2 {
					t.Fatal("analyzer error was hidden", err, r)
				}
			}
		})
	}
}
func TestRuleValidationAndDeterminism(t *testing.T) {
	s := load(t, "go")
	p := Policy{Default: "deny", Unresolved: "error", Rules: []Rule{grant("seam:world.reader", "allow"), grant("module:world", "deny")}}
	a, err := Evaluate(s, p)
	if err != nil {
		t.Fatal(err)
	}
	p.Rules[0], p.Rules[1] = p.Rules[1], p.Rules[0]
	b, err := Evaluate(s, p)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("rule order affected result", err)
	}
	for _, bad := range []Policy{{Default: "ignore", Unresolved: "error"}, {Default: "deny", Unresolved: "ignore"}, {Default: "deny", Unresolved: "error", Rules: []Rule{grant("module:missing", "allow")}}, {Default: "deny", Unresolved: "error", Rules: []Rule{grant("module:world", "allow"), grant("module:world", "deny")}}} {
		if _, err := Evaluate(s, bad); err == nil {
			t.Fatal("invalid policy accepted")
		}
	}
	s.Relations[0].Evidence = nil
	if _, err := Evaluate(s, p); err == nil {
		t.Fatal("invalid IR accepted")
	}
}
func TestMultipleSeamsAndInternalRelations(t *testing.T) {
	s := load(t, "go")
	other := ir.Seam{ID: "seam:world.other", ModuleID: "module:world", ContractMembers: append([]string{}, s.Seams[0].ContractMembers...)}
	s.Modules[1].Seams = append(s.Modules[1].Seams, other.ID)
	s.Seams = append(s.Seams, other)
	r, err := Evaluate(s, Policy{Default: "deny", Unresolved: "error", Rules: []Rule{grant("seam:world.reader", "allow"), grant(other.ID, "deny")}})
	if err != nil || r.ExitCode() != 1 {
		t.Fatal("overlapping seam deny ignored", r, err)
	}
	s.Relations[0].FromID = "module:world"
	x := &s.Relations[0]
	x.ID = ir.RelationID(x.Kind, x.FromID, x.ToID)
	r, err = Evaluate(s, Policy{Default: "deny", Unresolved: "error"})
	if err != nil || r.ExitCode() != 0 || len(r.Decisions) != 0 {
		t.Fatal("internal relation treated as consumer", r, err)
	}
}

func TestViolationIdentitySurvivesLocationChanges(t *testing.T) {
	s := load(t, "go")
	p := Policy{Default: "deny", Unresolved: "error"}
	before, err := Evaluate(s, p)
	if err != nil {
		t.Fatal(err)
	}
	s.Relations[0].Evidence[0].StartLine = 100
	s.Relations[0].Evidence[0].EndLine = 100
	after, err := Evaluate(s, p)
	if err != nil {
		t.Fatal(err)
	}
	if before.Violations[0].ID != after.Violations[0].ID {
		t.Fatal("source move changed violation identity")
	}
	if after.Violations[0].Evidence[0].StartLine != 100 {
		t.Fatal("stale source evidence")
	}
}

func TestStructuralSatisfactionIsNotConsumerUsage(t *testing.T) {
	s := load(t, "go")
	s.Relations[0].Kind = "implements"
	r := &s.Relations[0]
	r.ID = ir.RelationID(r.Kind, r.FromID, r.ToID)
	got, err := Evaluate(s, Policy{Default: "deny", Unresolved: "error"})
	if err != nil || got.ExitCode() != 0 || len(got.Decisions) != 0 {
		t.Fatal("structural compatibility invented a consumer dependency", got, err)
	}
}
