// Package report renders language-neutral check results.
package report

import (
	"encoding/json"
	"fmt"
	"io"

	"seamgraph/internal/policy"
)

func Write(w io.Writer, r policy.Result, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	case "text":
		for _, v := range r.Violations {
			where := ""
			if len(v.Evidence) > 0 {
				e := v.Evidence[0]
				where = fmt.Sprintf("%s:%d:%d: ", e.File, e.StartLine, e.StartColumn)
			}
			if _, err := fmt.Fprintf(w, "%s%s [%s]\n  %s\n", where, v.Message, v.ID, v.Remediation); err != nil {
				return err
			}
		}
		for _, d := range r.Diagnostics {
			if _, err := fmt.Fprintf(w, "%s %s: %s\n", d.Severity, d.Code, d.Message); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(w, "%d architecture violations; analysis errors: %t\n", len(r.Violations), r.Incomplete)
		return err
	case "sarif":
		results := []any{}
		for _, v := range r.Violations {
			locations := []any{}
			for _, e := range v.Evidence {
				locations = append(locations, map[string]any{"physicalLocation": map[string]any{"artifactLocation": map[string]any{"uri": e.File}, "region": map[string]any{"startLine": e.StartLine, "startColumn": e.StartColumn, "endLine": e.EndLine, "endColumn": e.EndColumn}}})
			}
			results = append(results, map[string]any{"ruleId": v.Code, "level": "error", "message": map[string]string{"text": v.Message + " " + v.Remediation}, "locations": locations, "partialFingerprints": map[string]string{"seamgraph/v1": v.ID}})
		}
		for _, d := range r.Diagnostics {
			level := d.Severity
			if level == "info" {
				level = "note"
			}
			results = append(results, map[string]any{"ruleId": d.Code, "level": level, "message": map[string]string{"text": d.Message}})
		}
		return json.NewEncoder(w).Encode(map[string]any{"version": "2.1.0", "$schema": "https://json.schemastore.org/sarif-2.1.0.json", "runs": []any{map[string]any{"tool": map[string]any{"driver": map[string]string{"name": "SeamGraph"}}, "results": results}}})
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

// ApplyBaseline suppresses only exact stable violation identities. Analyzer
// diagnostics are retained so a baseline cannot hide incomplete analysis.
func ApplyBaseline(r policy.Result, ids []string) policy.Result {
	known := map[string]bool{}
	for _, id := range ids {
		known[id] = true
	}
	out := []policy.Violation{}
	for _, v := range r.Violations {
		if !known[v.ID] {
			out = append(out, v)
		}
	}
	r.Violations = out
	return r
}
