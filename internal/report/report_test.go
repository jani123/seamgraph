package report

import (
	"bytes"
	"encoding/json"
	"testing"

	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
)

func TestFormatsAndBaselinePreserveErrors(t *testing.T) {
	r := policy.Result{SchemaVersion: 1, Incomplete: true, Violations: []policy.Violation{{ID: "violation:stable", Code: "SEAM_BYPASS", Message: "forbidden", Remediation: "use seam", Evidence: []ir.Evidence{{File: "src/main.go", StartLine: 2, StartColumn: 1, EndLine: 2, EndColumn: 3}}}}, Diagnostics: []ir.Diagnostic{{Code: "UNRESOLVED", Severity: "error", Message: "incomplete"}}}
	for _, format := range []string{"text", "json", "sarif"} {
		var b bytes.Buffer
		if err := Write(&b, r, format); err != nil {
			t.Fatal(err)
		}
		if format != "text" {
			var value any
			if err := json.Unmarshal(b.Bytes(), &value); err != nil {
				t.Fatal(err)
			}
		}
	}
	r = ApplyBaseline(r, []string{"violation:stable"})
	if r.ExitCode() != 2 || len(r.Violations) != 0 || len(r.Diagnostics) != 1 {
		t.Fatal("baseline hid incomplete analysis", r)
	}
	if err := Write(&bytes.Buffer{}, r, "unknown"); err == nil {
		t.Fatal("unknown format accepted")
	}
}
