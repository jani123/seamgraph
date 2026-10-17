// Package testresults adapts actual runner reports into verification evidence.
package testresults

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"

	"seamgraph/internal/ir"
	"seamgraph/internal/verification"
)

type Run struct{ Runner, Revision, Fingerprint, RunID, ObservedAt string }

func Import(format string, data []byte, run Run) ([]verification.Evidence, error) {
	outcomes := map[string]string{}
	switch format {
	case "go-test-json":
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		scanner.Buffer(make([]byte, 4096), 16*1024*1024)
		for scanner.Scan() {
			var e struct{ Action, Package, Test string }
			if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
				return nil, err
			}
			if e.Test != "" {
				outcome := map[string]string{"pass": "passed", "fail": "failed", "skip": "skipped"}[e.Action]
				if outcome != "" {
					outcomes[e.Package+"/"+e.Test] = outcome
				}
			} else if e.Action == "fail" {
				outcomes[e.Package+"/<package>"] = "failed"
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	case "junit":
		type Case struct {
			Name    string  `xml:"name,attr"`
			Class   string  `xml:"classname,attr"`
			Failure *string `xml:"failure"`
			Error   *string `xml:"error"`
			Skipped *string `xml:"skipped"`
		}
		type Suite struct {
			Cases []Case `xml:"testcase"`
		}
		var suites struct {
			XMLName xml.Name
			Suites  []Suite `xml:"testsuite"`
			Cases   []Case  `xml:"testcase"`
		}
		if err := xml.Unmarshal(data, &suites); err != nil {
			return nil, err
		}
		all := suites.Cases
		for _, s := range suites.Suites {
			all = append(all, s.Cases...)
		}
		for _, c := range all {
			id := c.Class + "/" + c.Name
			if _, ok := outcomes[id]; ok {
				return nil, fmt.Errorf("ambiguous JUnit test ID %s", id)
			}
			outcome := "passed"
			if c.Failure != nil || c.Error != nil {
				outcome = "failed"
			}
			if c.Skipped != nil {
				outcome = "skipped"
			}
			outcomes[id] = outcome
		}
	case "vitest-json", "jest-json":
		var report struct {
			TestResults []struct {
				Name             string `json:"name"`
				AssertionResults []struct {
					FullName string `json:"fullName"`
					Status   string `json:"status"`
				} `json:"assertionResults"`
			} `json:"testResults"`
		}
		if err := json.Unmarshal(data, &report); err != nil {
			return nil, err
		}
		for _, suite := range report.TestResults {
			for _, test := range suite.AssertionResults {
				outcome := map[string]string{"passed": "passed", "failed": "failed", "pending": "skipped", "skipped": "skipped", "todo": "skipped"}[test.Status]
				if outcome == "" {
					outcome = "unknown"
				}
				id := test.FullName
				if _, ok := outcomes[id]; ok {
					return nil, fmt.Errorf("ambiguous test fullName %q", id)
				}
				outcomes[id] = outcome
			}
		}
	default:
		return nil, fmt.Errorf("unsupported test artifact format %q", format)
	}
	if len(outcomes) == 0 {
		return nil, fmt.Errorf("artifact contains no test outcomes")
	}
	evidence := []verification.Evidence{}
	for id, outcome := range outcomes {
		evidence = append(evidence, verification.Evidence{ID: ir.ViolationID("test:"+run.RunID, id), RequirementIDs: []string{}, MemberIDs: []string{}, TestID: id, Runner: run.Runner, Revision: run.Revision, Fingerprint: run.Fingerprint, RunID: run.RunID, ObservedAt: run.ObservedAt, Outcome: outcome, EvidenceKind: "test-execution"})
	}
	return evidence, nil
}
