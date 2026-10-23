package main

import (
	"encoding/json"
	"fmt"
	"os"
	"seamgraph/internal/application"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"seamgraph/internal/report"
	"seamgraph/internal/verification"
	"strings"
)

func (c *commandInvocation) runScan(session *application.InspectionSession) int {
	s, result := session.Snapshot, session.Check
	if c.command == "snapshot" && c.opts.output == "" {
		return c.fail(fmt.Errorf("snapshot requires --output FILE"))
	}
	if c.command == "snapshot" || c.opts.format == "json" {
		b, err := s.JSON()
		if err != nil {
			return c.fail(err)
		}
		if _, err = c.out.Write(b); err != nil {
			return c.fail(err)
		}
	} else if c.opts.format == "text" {
		_, _ = fmt.Fprintf(c.out, "%d modules, %d seams, %d symbols, %d observed relations\n", len(s.Modules), len(s.Seams), len(s.Symbols), len(s.Relations))
		for _, m := range s.Modules {
			_, _ = fmt.Fprintf(c.out, "%s (%s)\n", m.ID, strings.Join(m.SourceRoots, ", "))
		}
		for _, d := range result.Diagnostics {
			_, _ = fmt.Fprintf(c.out, "%s %s: %s\n", d.Severity, d.Code, d.Message)
		}
	} else {
		return c.fail(fmt.Errorf("scan supports text or json"))
	}
	if result.Incomplete {
		return 2
	}
	return 0
}

func (c *commandInvocation) runCheck(session *application.InspectionSession) int {
	result := session.Check
	verify := func() (verification.Report, error) { return session.Verify(c.ctx) }
	var err error
	if c.opts.baseline != "" {
		// #nosec G703 -- This local CLI operation intentionally uses the path explicitly selected by the user.
		b, err := os.ReadFile(c.opts.baseline)
		if err != nil {
			return c.fail(err)
		}
		var ids []string
		if err = json.Unmarshal(b, &ids); err != nil {
			return c.fail(err)
		}
		result = report.ApplyBaseline(result, ids)
	}
	if c.opts.contracts {
		cr, err := verify()
		if err != nil {
			return c.fail(err)
		}
		if c.opts.format != "json" {
			return c.fail(fmt.Errorf("contract checking currently requires --json"))
		}
		if err = writeJSON(c.out, map[string]any{"schemaVersion": 1, "architecture": result, "contracts": cr}); err != nil {
			return c.fail(err)
		}
		return max(result.ExitCode(), cr.ExitCode)
	}

	if err = report.Write(c.out, result, c.opts.format); err != nil {
		return c.fail(err)
	}
	return result.ExitCode()
}

func (c *commandInvocation) runInspect(session *application.InspectionSession) int {
	s, result := session.Snapshot, session.Check
	verify := func() (verification.Report, error) { return session.Verify(c.ctx) }
	if c.inspectID == "" {
		return c.fail(fmt.Errorf("inspect requires a stable architecture ID"))
	}
	inspection, err := application.Inspect(s, result, c.inspectID)
	if err != nil {
		return c.fail(err)
	}
	entity, related := inspection.Entity, inspection.Relations
	var cr *verification.Report
	if c.opts.tests {
		r, err := verify()
		if err != nil {
			return c.fail(err)
		}
		cr = &r
	}
	if c.opts.format == "text" {
		c.writeInspection(s, inspection, cr)
		if result.Incomplete {
			return 2
		}
		return 0
	}
	if c.opts.format != "json" {
		return c.fail(fmt.Errorf("inspect supports text or json"))
	}
	returnErr := writeJSON(c.out, map[string]any{"contracts": cr, "schemaVersion": 1, "entity": entity, "relations": related, "diagnostics": result.Diagnostics, "violations": result.Violations})
	if returnErr != nil {
		return c.fail(returnErr)
	}
	if result.Incomplete {
		return 2
	}
	return 0
}

func (c *commandInvocation) writeInspection(s ir.Snapshot, inspection application.Inspection, cr *verification.Report) {
	_, _ = fmt.Fprintln(c.out, c.inspectID)
	c.writeEntity(s, inspection.Entity)
	for _, r := range inspection.Relations {
		_, _ = fmt.Fprintln(c.out, r.Kind, r.FromID, "->", r.ToID)
		for _, e := range r.Evidence {
			_, _ = fmt.Fprintf(c.out, "  %s:%d:%d\n", e.File, e.StartLine, e.StartColumn)
		}
	}
	if cr != nil {
		for _, r := range cr.Results {
			if r.SeamID == c.inspectID {
				_, _ = fmt.Fprintf(c.out, "Contract status: %s; member execution coverage: %d/%d; declared requirement verification: %d/%d\n", r.Status, r.MemberCoverage.Covered, r.MemberCoverage.Total, r.RequirementCoverage.Covered, r.RequirementCoverage.Total)
			}
		}
	}
}

func (c *commandInvocation) writeEntity(s ir.Snapshot, entity any) {
	switch x := entity.(type) {
	case ir.Module:
		_, _ = fmt.Fprintln(c.out, "Roots:", strings.Join(x.SourceRoots, ", "))
		for _, id := range x.Seams {
			_, _ = fmt.Fprintln(c.out, "Seam:", id)
		}
	case ir.Seam:
		for _, symbol := range s.Symbols {
			for _, id := range x.ContractMembers {
				if symbol.ID == id {
					_, _ = fmt.Fprintln(c.out, symbol.ID, symbol.DisplaySignature)
				}
			}
		}
	case ir.Symbol:
		_, _ = fmt.Fprintln(c.out, x.DisplaySignature)
	case ir.Relation:
		_, _ = fmt.Fprintln(c.out, x.Kind, x.FromID, "->", x.ToID)
	case policy.Violation:
		_, _ = fmt.Fprintln(c.out, x.Message, x.Remediation)
	}
}
