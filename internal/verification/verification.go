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
	requirements := map[string]Requirement{}
	for _, r := range m.Requirements {
		seam, ok := seams[r.SeamID]
		if !ok || r.ID == "" || r.Description == "" || r.Kind == "" {
			return report, fmt.Errorf("invalid contract requirement %q", r.ID)
		}
		if _, ok := requirements[r.ID]; ok {
			return report, fmt.Errorf("duplicate requirement %s", r.ID)
		}
		if r.MemberID != "" && !slices.Contains(seam.ContractMembers, r.MemberID) {
			return report, fmt.Errorf("requirement %s has unknown seam member", r.ID)
		}
		requirements[r.ID] = r
	}
	links := map[string]Link{}
	for _, l := range m.Links {
		if l.TestID == "" {
			return report, fmt.Errorf("empty test ID")
		}
		if _, ok := links[l.TestID]; ok {
			return report, fmt.Errorf("duplicate test mapping %s", l.TestID)
		}
		for _, id := range l.RequirementIDs {
			if _, ok := requirements[id]; !ok {
				return report, fmt.Errorf("unknown linked requirement %s", id)
			}
		}
		for _, id := range l.MemberIDs {
			if !members[id] {
				return report, fmt.Errorf("unknown linked seam member %s", id)
			}
		}
		links[l.TestID] = l
	}
	for id, mode := range m.Policies {
		if _, ok := seams[id]; !ok {
			return report, fmt.Errorf("unknown verification policy seam %s", id)
		}
		if !slices.Contains([]string{"report", "fail-on-failed", "fail-on-missing", "require-verified"}, mode) {
			return report, fmt.Errorf("unknown verification policy %q", mode)
		}
	}
	evidence := append([]Evidence{}, store.Evidence...)
	seen := map[string]bool{}
	for i := range evidence {
		e := &evidence[i]
		if e.ID == "" || seen[e.ID] || e.TestID == "" || e.RunID == "" || e.Runner == "" || e.ObservedAt == "" || !slices.Contains([]string{"passed", "failed", "skipped", "unknown"}, e.Outcome) || !slices.Contains([]string{"test-execution", "coverage", "static-linkage", "reviewed-assertion"}, e.EvidenceKind) {
			return report, fmt.Errorf("invalid test evidence %q", e.ID)
		}
		if _, err := time.Parse(time.RFC3339Nano, e.ObservedAt); err != nil {
			return report, fmt.Errorf("evidence %s has invalid observedAt", e.ID)
		}
		if e.SourceLocation != nil {
			if err := e.SourceLocation.Validate(); err != nil {
				return report, err
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
				return report, fmt.Errorf("coverage references unknown member %s", id)
			}
		}
		if l, ok := links[e.TestID]; ok {
			if e.EvidenceKind != "coverage" {
				e.MemberIDs = append(e.MemberIDs, l.MemberIDs...)
			}
			if l.SourceLocation != nil {
				if err := l.SourceLocation.Validate(); err != nil {
					return report, err
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
	}
	slices.SortFunc(evidence, func(a, b Evidence) int { return strings.Compare(a.ID, b.ID) })
	report.Evidence = evidence
	report.Requirements = append(report.Requirements, m.Requirements...)
	slices.SortFunc(report.Requirements, func(a, b Requirement) int { return strings.Compare(a.ID, b.ID) })
	// Only the most recent execution of a test/evidence kind is authoritative.
	latest := map[string]Evidence{}
	outcomeRank := func(outcome string) int {
		switch outcome {
		case "failed":
			return 3
		case "unknown", "skipped":
			return 2
		default:
			return 1
		}
	}
	for _, e := range evidence {
		key := e.TestID + "\x00" + e.EvidenceKind
		old, ok := latest[key]
		a, _ := time.Parse(time.RFC3339Nano, e.ObservedAt)
		b, _ := time.Parse(time.RFC3339Nano, old.ObservedAt)
		current := e.Revision == revision && e.Fingerprint == fingerprint
		oldCurrent := old.Revision == revision && old.Fingerprint == fingerprint
		if !ok || (current && !oldCurrent) || (current == oldCurrent && (a.After(b) || (a.Equal(b) && (outcomeRank(e.Outcome) > outcomeRank(old.Outcome) || (outcomeRank(e.Outcome) == outcomeRank(old.Outcome) && e.ID > old.ID))))) {
			latest[key] = e
		}
	}
	for _, seam := range c.Seams {
		r := Result{SeamID: seam.ID, Revision: revision, Status: "unverified", MemberCoverage: Coverage{Total: len(seam.ContractMembers)}, MissingRequirements: []string{}, FailingTests: []string{}, StaleEvidence: []string{}, UnknownEvidence: []string{}}
		required := []Requirement{}
		for _, req := range report.Requirements {
			if req.SeamID == seam.ID {
				required = append(required, req)
			}
		}
		r.RequirementCoverage.Total = len(required)
		r.RequirementCoverage.Known = true
		coveredMembers := map[string]bool{}
		passed := map[string]bool{}
		hasUnknown := false
		for _, e := range evidence {
			if latest[e.TestID+"\x00"+e.EvidenceKind].ID != e.ID {
				continue
			}
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
			if !relevant {
				continue
			}
			if e.Revision != revision || e.Fingerprint != fingerprint || revision == "" || fingerprint == "" {
				r.StaleEvidence = append(r.StaleEvidence, e.ID)
				continue
			}
			if e.EvidenceKind == "coverage" && e.Outcome == "passed" {
				for _, id := range e.MemberIDs {
					if slices.Contains(seam.ContractMembers, id) {
						coveredMembers[id] = true
					}
				}
				continue
			}
			if e.EvidenceKind != "test-execution" {
				r.UnknownEvidence = append(r.UnknownEvidence, e.ID)
				continue
			}
			if e.Outcome == "failed" {
				r.FailingTests = append(r.FailingTests, e.TestID)
				continue
			}
			if e.Outcome != "passed" {
				hasUnknown = true
				r.UnknownEvidence = append(r.UnknownEvidence, e.ID)
				continue
			}
			for _, id := range e.RequirementIDs {
				if requirements[id].SeamID == seam.ID {
					passed[id] = true
				}
			}
		}
		r.MemberCoverage.Covered = len(coveredMembers)
		r.MemberCoverage.Known = len(coveredMembers) > 0 && len(coveredMembers) == len(seam.ContractMembers)
		for _, req := range required {
			if passed[req.ID] {
				r.RequirementCoverage.Covered++
			} else {
				r.MissingRequirements = append(r.MissingRequirements, req.ID)
				linked := false
				for _, l := range m.Links {
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
		mode := m.Policies[seam.ID]
		if mode != "" && mode != "report" && (r.Status == "unknown" || r.Status == "stale") {
			report.ExitCode = 2
		}
		violates := mode == "require-verified" && r.Status != "verified" || mode == "fail-on-missing" && (len(r.MissingRequirements) > 0 || len(required) == 0 || r.Status != "verified") || mode == "fail-on-failed" && r.Status == "failed"
		if violates {
			code := 1
			if r.Status == "unknown" || r.Status == "stale" {
				code = 2
			}
			if code > report.ExitCode {
				report.ExitCode = code
			}
		}
		report.Results = append(report.Results, r)
	}
	return report, nil
}
