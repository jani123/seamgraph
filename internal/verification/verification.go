// Package verification assesses explicitly declared contract behavior against
// normalized evidence. It never infers assertions from test names or coverage.
package verification

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"seamgraph/internal/ir"
)

type Requirement struct {
	ID          string `json:"id" yaml:"id"`
	SeamID      string `json:"seamId" yaml:"seamId"`
	MemberID    string `json:"memberId,omitempty" yaml:"memberId,omitempty"`
	Description string `json:"description" yaml:"description"`
	Kind        string `json:"kind" yaml:"kind"`
}
type Link struct {
	TestID         string       `json:"testId" yaml:"testId"`
	RequirementIDs []string     `json:"requirementIds" yaml:"requirementIds"`
	MemberIDs      []string     `json:"memberIds" yaml:"memberIds"`
	Reviewed       bool         `json:"reviewed" yaml:"reviewed"`
	SourceLocation *ir.Evidence `json:"sourceLocation,omitempty" yaml:"sourceLocation,omitempty"`
}
type Evidence struct {
	ID             string       `json:"id"`
	RequirementIDs []string     `json:"requirementIds"`
	MemberIDs      []string     `json:"memberIds"`
	TestID         string       `json:"testId"`
	Runner         string       `json:"runner"`
	SourceLocation *ir.Evidence `json:"sourceLocation,omitempty"`
	Revision       string       `json:"revision"`
	Fingerprint    string       `json:"fingerprint"`
	RunID          string       `json:"runId"`
	Outcome        string       `json:"outcome"`
	ObservedAt     string       `json:"observedAt"`
	EvidenceKind   string       `json:"evidenceKind"`
}
type Manifest struct {
	Version      int               `yaml:"version" json:"version"`
	Requirements []Requirement     `yaml:"requirements" json:"requirements"`
	Links        []Link            `yaml:"links" json:"links"`
	Policies     map[string]string `yaml:"policies" json:"policies"`
}
type Store struct {
	SchemaVersion int        `json:"schemaVersion"`
	Evidence      []Evidence `json:"evidence"`
}
type Coverage struct {
	Known   bool `json:"known"`
	Covered int  `json:"covered"`
	Total   int  `json:"total"`
}
type Result struct {
	SeamID              string   `json:"seamId"`
	Revision            string   `json:"revision"`
	Status              string   `json:"status"`
	Code                string   `json:"code"`
	MemberCoverage      Coverage `json:"memberCoverage"`
	RequirementCoverage Coverage `json:"requirementCoverage"`
	MissingRequirements []string `json:"missingRequirements"`
	FailingTests        []string `json:"failingTests"`
	StaleEvidence       []string `json:"staleEvidence"`
	UnknownEvidence     []string `json:"unknownEvidence"`
}
type Report struct {
	SchemaVersion int           `json:"schemaVersion"`
	Results       []Result      `json:"results"`
	Requirements  []Requirement `json:"requirements"`
	Evidence      []Evidence    `json:"evidence"`
	ExitCode      int           `json:"exitCode"`
}

func Evaluate(s ir.Snapshot, m Manifest, store Store, revision, fingerprint string) (Report, error) {
	report := Report{SchemaVersion: 1, Results: []Result{}, Requirements: []Requirement{}, Evidence: []Evidence{}}
	c, err := s.Canonical()
	if err != nil {
		return report, err
	}
	if m.Version != 1 || store.SchemaVersion != 1 {
		return report, fmt.Errorf("unsupported contract manifest/evidence version")
	}
	seams := map[string]ir.Seam{}
	members := map[string]bool{}
	for _, seam := range c.Seams {
		seams[seam.ID] = seam
		for _, id := range seam.ContractMembers {
			members[id] = true
		}
	}
	requirements, err := validateRequirements(m.Requirements, seams)
	if err != nil {
		return report, err
	}
	links, err := validateLinks(m.Links, requirements, members)
	if err != nil {
		return report, err
	}
	if err := validatePolicies(m.Policies, seams); err != nil {
		return report, err
	}
	evidence, err := prepareEvidence(store, links, members)
	if err != nil {
		return report, err
	}
	slices.SortFunc(evidence, func(a, b Evidence) int { return strings.Compare(a.ID, b.ID) })
	report.Evidence = evidence
	report.Requirements = append(report.Requirements, m.Requirements...)
	slices.SortFunc(report.Requirements, func(a, b Requirement) int { return strings.Compare(a.ID, b.ID) })
	evaluator := seamEvaluator{
		declared: report.Requirements, requirements: requirements, links: m.Links,
		evidence: evidence, latest: latestEvidence(evidence, revision, fingerprint),
		revision: revision, fingerprint: fingerprint,
	}
	for _, seam := range c.Seams {
		r := evaluator.evaluate(seam)
		report.ExitCode = max(report.ExitCode, policyExitCode(m.Policies[seam.ID], r))
		report.Results = append(report.Results, r)
	}
	return report, nil
}

func validateRequirements(declared []Requirement, seams map[string]ir.Seam) (map[string]Requirement, error) {
	requirements := map[string]Requirement{}
	for _, r := range declared {
		seam, ok := seams[r.SeamID]
		if !ok || r.ID == "" || r.Description == "" || r.Kind == "" {
			return nil, fmt.Errorf("invalid contract requirement %q", r.ID)
		}
		if _, ok := requirements[r.ID]; ok {
			return nil, fmt.Errorf("duplicate requirement %s", r.ID)
		}
		if r.MemberID != "" && !slices.Contains(seam.ContractMembers, r.MemberID) {
			return nil, fmt.Errorf("requirement %s has unknown seam member", r.ID)
		}
		requirements[r.ID] = r
	}
	return requirements, nil
}

func validateLinks(declared []Link, requirements map[string]Requirement, members map[string]bool) (map[string]Link, error) {
	links := map[string]Link{}
	for _, l := range declared {
		if l.TestID == "" {
			return nil, fmt.Errorf("empty test ID")
		}
		if _, ok := links[l.TestID]; ok {
			return nil, fmt.Errorf("duplicate test mapping %s", l.TestID)
		}
		for _, id := range l.RequirementIDs {
			if _, ok := requirements[id]; !ok {
				return nil, fmt.Errorf("unknown linked requirement %s", id)
			}
		}
		for _, id := range l.MemberIDs {
			if !members[id] {
				return nil, fmt.Errorf("unknown linked seam member %s", id)
			}
		}
		links[l.TestID] = l
	}
	return links, nil
}

func validatePolicies(policies map[string]string, seams map[string]ir.Seam) error {
	for id, mode := range policies {
		if _, ok := seams[id]; !ok {
			return fmt.Errorf("unknown verification policy seam %s", id)
		}
		if !slices.Contains([]string{"report", "fail-on-failed", "fail-on-missing", "require-verified"}, mode) {
			return fmt.Errorf("unknown verification policy %q", mode)
		}
	}
	return nil
}

func prepareEvidence(store Store, links map[string]Link, members map[string]bool) ([]Evidence, error) {
	evidence := append([]Evidence{}, store.Evidence...)
	seen := map[string]bool{}
	for i := range evidence {
		if err := prepareEvidenceEntry(&evidence[i], seen, links, members); err != nil {
			return nil, err
		}
	}
	return evidence, nil
}

func prepareEvidenceEntry(e *Evidence, seen map[string]bool, links map[string]Link, members map[string]bool) error {
	if e.ID == "" || seen[e.ID] || e.TestID == "" || e.RunID == "" || e.Runner == "" || e.ObservedAt == "" || !slices.Contains([]string{"passed", "failed", "skipped", "unknown"}, e.Outcome) || !slices.Contains([]string{"test-execution", "coverage", "static-linkage", "reviewed-assertion"}, e.EvidenceKind) {
		return fmt.Errorf("invalid test evidence %q", e.ID)
	}
	if _, err := time.Parse(time.RFC3339Nano, e.ObservedAt); err != nil {
		return fmt.Errorf("evidence %s has invalid observedAt", e.ID)
	}
	if e.SourceLocation != nil {
		if err := e.SourceLocation.Validate(); err != nil {
			return err
		}
	}
	seen[e.ID] = true
	// Imported outcomes never invent mappings. Only the current explicit manifest
	// supplies member/requirement links; evidence cannot grant itself coverage.
	e.RequirementIDs = []string{}
	if e.EvidenceKind != "coverage" {
		e.MemberIDs = []string{}
	}
	for _, id := range e.MemberIDs {
		if !members[id] {
			return fmt.Errorf("coverage references unknown member %s", id)
		}
	}
	if l, ok := links[e.TestID]; ok {
		if e.EvidenceKind != "coverage" {
			e.MemberIDs = append(e.MemberIDs, l.MemberIDs...)
		}
		if l.SourceLocation != nil {
			if err := l.SourceLocation.Validate(); err != nil {
				return err
			}
			copy := *l.SourceLocation
			e.SourceLocation = &copy
		}
		if l.Reviewed {
			e.RequirementIDs = append(e.RequirementIDs, l.RequirementIDs...)
		}
	}
	slices.Sort(e.RequirementIDs)
	slices.Sort(e.MemberIDs)
	return nil
}

func latestEvidence(evidence []Evidence, revision, fingerprint string) map[string]Evidence {
	// Only the most recent execution of a test/evidence kind is authoritative.
	latest := map[string]Evidence{}

	for _, e := range evidence {
		key := e.TestID + "\x00" + e.EvidenceKind
		old, ok := latest[key]
		if !ok || newerEvidence(e, old, revision, fingerprint) {
			latest[key] = e
		}
	}
	return latest
}

func newerEvidence(e, old Evidence, revision, fingerprint string) bool {
	a, _ := time.Parse(time.RFC3339Nano, e.ObservedAt)
	b, _ := time.Parse(time.RFC3339Nano, old.ObservedAt)
	current := e.Revision == revision && e.Fingerprint == fingerprint
	oldCurrent := old.Revision == revision && old.Fingerprint == fingerprint
	if current != oldCurrent {
		return current
	}
	if !a.Equal(b) {
		return a.After(b)
	}
	if outcomeRank(e.Outcome) != outcomeRank(old.Outcome) {
		return outcomeRank(e.Outcome) > outcomeRank(old.Outcome)
	}
	return e.ID > old.ID
}

func (v seamEvaluator) evaluate(seam ir.Seam) Result {
	r := Result{SeamID: seam.ID, Revision: v.revision, Status: "unverified", MemberCoverage: Coverage{Total: len(seam.ContractMembers)}, MissingRequirements: []string{}, FailingTests: []string{}, StaleEvidence: []string{}, UnknownEvidence: []string{}}
	required := []Requirement{}
	for _, req := range v.declared {
		if req.SeamID == seam.ID {
			required = append(required, req)
		}
	}
	r.RequirementCoverage.Total = len(required)
	r.RequirementCoverage.Known = true
	coveredMembers, passed, hasUnknown := v.evaluateEvidence(&r, seam)
	r.MemberCoverage.Covered = len(coveredMembers)
	r.MemberCoverage.Known = len(coveredMembers) > 0 && len(coveredMembers) == len(seam.ContractMembers)
	for _, req := range required {
		if passed[req.ID] {
			r.RequirementCoverage.Covered++
		} else {
			r.MissingRequirements = append(r.MissingRequirements, req.ID)
			linked := false
			for _, l := range v.links {
				if l.Reviewed && slices.Contains(l.RequirementIDs, req.ID) {
					linked = true
				}
			}
			if linked {
				hasUnknown = true
			}
		}
	}
	switch {
	case len(r.FailingTests) > 0:
		r.Status = "failed"
	case len(required) > 0 && len(r.MissingRequirements) == 0:
		r.Status = "verified"
	case len(r.StaleEvidence) > 0:
		r.Status = "stale"
	case r.RequirementCoverage.Covered > 0:
		r.Status = "partial"
	case hasUnknown || len(r.UnknownEvidence) > 0:
		r.Status = "unknown"
	}
	r.Code = "CONTRACT_" + strings.ToUpper(r.Status)
	for _, ss := range []*[]string{&r.FailingTests, &r.StaleEvidence, &r.UnknownEvidence} {
		slices.Sort(*ss)
		*ss = slices.Compact(*ss)
	}
	return r
}

func (v seamEvaluator) evaluateEvidence(r *Result, seam ir.Seam) (map[string]bool, map[string]bool, bool) {
	coveredMembers := map[string]bool{}
	passed := map[string]bool{}
	hasUnknown := false
	for _, e := range v.evidence {
		if v.evaluateEvidenceEntry(r, seam, e, coveredMembers, passed) {
			hasUnknown = true
		}
	}
	return coveredMembers, passed, hasUnknown
}

func (v seamEvaluator) evaluateEvidenceEntry(r *Result, seam ir.Seam, e Evidence, coveredMembers, passed map[string]bool) bool {
	if v.latest[e.TestID+"\x00"+e.EvidenceKind].ID != e.ID {
		return false
	}
	relevant := evidenceRelevant(e, seam, v.requirements)
	if !relevant {
		return false
	}
	if e.Revision != v.revision || e.Fingerprint != v.fingerprint || v.revision == "" || v.fingerprint == "" {
		r.StaleEvidence = append(r.StaleEvidence, e.ID)
		return false
	}
	if e.EvidenceKind == "coverage" && e.Outcome == "passed" {
		for _, id := range e.MemberIDs {
			if slices.Contains(seam.ContractMembers, id) {
				coveredMembers[id] = true
			}
		}
		return false
	}
	if e.EvidenceKind != "test-execution" {
		r.UnknownEvidence = append(r.UnknownEvidence, e.ID)
		return false
	}
	if e.Outcome == "failed" {
		r.FailingTests = append(r.FailingTests, e.TestID)
		return false
	}
	if e.Outcome != "passed" {
		r.UnknownEvidence = append(r.UnknownEvidence, e.ID)
		return true
	}
	for _, id := range e.RequirementIDs {
		if v.requirements[id].SeamID == seam.ID {
			passed[id] = true
		}
	}
	return false
}

func evidenceRelevant(e Evidence, seam ir.Seam, requirements map[string]Requirement) bool {
	relevant := false
	for _, id := range e.MemberIDs {
		if slices.Contains(seam.ContractMembers, id) {
			relevant = true
		}
	}
	for _, id := range e.RequirementIDs {
		if requirements[id].SeamID == seam.ID {
			relevant = true
		}
	}
	return relevant
}

func policyExitCode(mode string, r Result) int {
	exitCode := 0
	if mode != "" && mode != "report" && (r.Status == "unknown" || r.Status == "stale") {
		exitCode = 2
	}
	violates := mode == "require-verified" && r.Status != "verified" || mode == "fail-on-missing" && (len(r.MissingRequirements) > 0 || r.RequirementCoverage.Total == 0 || r.Status != "verified") || mode == "fail-on-failed" && r.Status == "failed"
	if violates {
		code := 1
		if r.Status == "unknown" || r.Status == "stale" {
			code = 2
		}
		if code > exitCode {
			exitCode = code
		}
	}
	return exitCode
}

func outcomeRank(outcome string) int {
	switch outcome {
	case "failed":
		return 3
	case "unknown", "skipped":
		return 2
	default:
		return 1
	}
}

// seamEvaluator shares the validated evidence and identity across seam assessments.
// Per-seam coverage and outcomes stay local to each result.
type seamEvaluator struct {
	declared              []Requirement
	requirements          map[string]Requirement
	links                 []Link
	evidence              []Evidence
	latest                map[string]Evidence
	revision, fingerprint string
}
