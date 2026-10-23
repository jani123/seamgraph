// Package policy evaluates normalized architecture facts without language logic.
package policy

import (
	"fmt"
	"slices"
	"strings"

	"seamgraph/internal/ir"
)

type Rule struct {
	FromID string `json:"fromId"`
	ToID   string `json:"toId"`
	Effect string `json:"effect"`
}

func (r Rule) ID() string { return "rule:" + r.Effect + ":" + r.FromID + "->" + r.ToID }

type Policy struct {
	Default    string `json:"default"`
	Unresolved string `json:"unresolved"`
	Rules      []Rule `json:"rules"`
}
type Decision struct {
	RelationID string   `json:"relationId"`
	ConsumerID string   `json:"consumerId"`
	ProviderID string   `json:"providerId"`
	SeamIDs    []string `json:"seamIds"`
	Category   string   `json:"category"`
	Allowed    bool     `json:"allowed"`
	RuleID     string   `json:"ruleId"`
}
type Violation struct {
	ID          string        `json:"id"`
	RelationID  string        `json:"relationId"`
	ConsumerID  string        `json:"consumerId"`
	ProviderID  string        `json:"providerId"`
	SymbolID    string        `json:"symbolId,omitempty"`
	Code        string        `json:"code"`
	RuleID      string        `json:"ruleId"`
	Evidence    []ir.Evidence `json:"evidence"`
	Message     string        `json:"message"`
	Remediation string        `json:"remediation"`
}
type Result struct {
	SchemaVersion int             `json:"schemaVersion"`
	Decisions     []Decision      `json:"decisions"`
	Violations    []Violation     `json:"violations"`
	Diagnostics   []ir.Diagnostic `json:"diagnostics"`
	Incomplete    bool            `json:"incomplete"`
}

// ExitCode gives analysis errors precedence over architecture violations.
func (r Result) ExitCode() int {
	if r.Incomplete {
		return 2
	}
	if len(r.Violations) > 0 {
		return 1
	}
	return 0
}

func (p Policy) Validate(s ir.Snapshot) error {
	if p.Default != "allow" && p.Default != "deny" {
		return fmt.Errorf("policy default must be allow or deny")
	}
	if p.Unresolved != "error" && p.Unresolved != "warn" {
		return fmt.Errorf("unresolved policy must be error or warn")
	}
	modules := map[string]bool{}
	targets := map[string]bool{}
	for _, m := range s.Modules {
		modules[m.ID] = true
		targets[m.ID] = true
	}
	for _, x := range s.Seams {
		targets[x.ID] = true
	}
	seen := map[string]bool{}
	for _, r := range p.Rules {
		if !modules[r.FromID] || !targets[r.ToID] {
			return fmt.Errorf("unknown policy endpoint in %s", r.ID())
		}
		if r.Effect != "allow" && r.Effect != "deny" {
			return fmt.Errorf("invalid rule effect %q", r.Effect)
		}
		key := r.FromID + "->" + r.ToID
		if seen[key] {
			return fmt.Errorf("duplicate or conflicting rule %s", key)
		}
		seen[key] = true
	}
	return nil
}

func Evaluate(s ir.Snapshot, p Policy) (Result, error) {
	result := Result{SchemaVersion: 1, Decisions: []Decision{}, Violations: []Violation{}, Diagnostics: []ir.Diagnostic{}}
	c, err := s.Canonical()
	if err != nil {
		return result, err
	}
	if err = p.Validate(c); err != nil {
		return result, err
	}
	owners := ir.Owners(c)
	members := map[string][]string{}
	seams := map[string]string{}
	symbols := map[string]bool{}
	for _, x := range c.Seams {
		seams[x.ID] = x.ModuleID
		for _, id := range x.ContractMembers {
			members[id] = append(members[id], x.ID)
		}
	}
	for _, x := range c.Symbols {
		symbols[x.ID] = true
	}
	rules := append([]Rule{}, p.Rules...)
	slices.SortFunc(rules, func(a, b Rule) int { return strings.Compare(a.ID(), b.ID()) })
	for _, d := range c.Diagnostics {
		evaluateDiagnostic(&result, d, p.Unresolved)
	}
	for _, r := range c.Relations {
		from, to := owners[r.FromID], owners[r.ToID]
		if from == to || !ir.IsConsumerRelation(r) {
			continue
		}
		d := decideRelation(r, p, from, to, members, seams, symbols, rules)
		result.Decisions = append(result.Decisions, d)
		if !d.Allowed {
			result.Violations = append(result.Violations, deniedRelation(r, d, symbols[r.ToID]))
		}
	}
	slices.SortFunc(result.Violations, func(a, b Violation) int { return strings.Compare(a.ID, b.ID) })
	return result, nil
}

func evaluateDiagnostic(result *Result, d ir.Diagnostic, unresolved string) {
	uncertain := false
	for _, e := range d.Evidence {
		if e.Resolution != "resolved" {
			uncertain = true
		}
	}
	if uncertain && unresolved == "error" {
		d.Severity = "error"
	}
	if uncertain && unresolved == "warn" && d.Severity != "error" {
		d.Severity = "warning"
	}
	if d.Severity == "error" {
		result.Incomplete = true
	}
	result.Diagnostics = append(result.Diagnostics, d)
}

func decideRelation(r ir.Relation, p Policy, from, to string, members map[string][]string, seams map[string]string, symbols map[string]bool, rules []Rule) Decision {
	d := Decision{RelationID: r.ID, ConsumerID: from, ProviderID: to, SeamIDs: []string{}, Category: "seam-use", Allowed: p.Default == "allow", RuleID: "default:" + p.Default}
	if symbols[r.ToID] {
		d.SeamIDs = append(d.SeamIDs, members[r.ToID]...)
	} else if _, ok := seams[r.ToID]; ok {
		d.SeamIDs = append(d.SeamIDs, r.ToID)
	} else {
		d.Category = "dependency"
	}
	bypass := r.BypassesSeam || (symbols[r.ToID] && len(d.SeamIDs) == 0)
	if bypass {
		d.Category = "bypass"
		d.Allowed = false
		d.RuleID = "boundary:seam-required"
	}
	applyRules(&d, rules, seams, bypass)
	return d
}

func applyRules(d *Decision, rules []Rule, seams map[string]string, bypass bool) {
	// A provider import can be enabled by any grant to that provider's seams,
	// without claiming the import proves which seam members were used.
	var allowID, denyID string
	for _, rule := range rules {
		if rule.FromID != d.ConsumerID {
			continue
		}
		match := rule.ToID == d.ProviderID
		if d.Category == "dependency" {
			if rule.Effect == "allow" && seams[rule.ToID] == d.ProviderID {
				match = true
			}
		} else if slices.Contains(d.SeamIDs, rule.ToID) {
			match = true
		}
		if !match {
			continue
		}
		if rule.Effect == "deny" {
			if denyID == "" {
				denyID = rule.ID()
			}
		} else if !bypass && (d.Category == "dependency" || rule.ToID != d.ProviderID) {
			if allowID == "" {
				allowID = rule.ID()
			}
		}
	}
	if allowID != "" {
		d.Allowed = true
		d.RuleID = allowID
	}
	if denyID != "" {
		d.Allowed = false
		d.RuleID = denyID
	}
}

func deniedRelation(r ir.Relation, d Decision, symbol bool) Violation {
	code := "DEPENDENCY_DENIED"
	hint := "Declare a grant to the intended public seam, or remove the dependency."
	if d.Category == "seam-use" {
		code = "SEAM_USE_DENIED"
	}
	if d.Category == "bypass" {
		code = "SEAM_BYPASS"
		hint = "Use the provider's declared public seam and entrypoint instead of internal symbols or deep paths."
	}
	symbolID := ""
	if symbol {
		symbolID = r.ToID
	}
	violationEvidence := r.Evidence
	if len(r.BypassEvidence) > 0 {
		violationEvidence = r.BypassEvidence
	}
	return Violation{ID: ir.ViolationID(d.RuleID, r.ID), RelationID: r.ID, ConsumerID: d.ConsumerID, ProviderID: d.ProviderID, SymbolID: symbolID, Code: code, RuleID: d.RuleID, Evidence: violationEvidence, Message: fmt.Sprintf("%s cannot access %s (%s)", d.ConsumerID, r.ToID, code), Remediation: hint}
}
