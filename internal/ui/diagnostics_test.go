package ui

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"seamgraph/internal/application"
	"seamgraph/internal/ir"
)

func TestDiagnosticsConsoleCollectsFailuresAndFindings(t *testing.T) {
	browser := application.CodeBrowser{
		Issues: []string{"analyzer failed <script>bad()</script>", "analyzer failed <script>bad()</script>"},
		Diagnostics: []ir.Diagnostic{
			{Severity: "error", Code: "BROKEN", Message: "analyzer failed <script>bad()</script>"},
			{Severity: "warning", Code: "UNRESOLVED", Message: "Cannot resolve import", Evidence: []ir.Evidence{{File: "app.py", StartLine: 3, StartColumn: 1}}, EntityIDs: []string{"symbol:app.run"}},
			{Severity: "info", Code: "INFO", Message: "Some context"},
		},
	}
	entries := codeDiagnostics(browser, CodeFilter{ReviewIssue: "No architecture configuration"})
	if len(entries) != 4 {
		t.Fatalf("expected four unique entries, got %v", entries)
	}
	if label := consoleLabel(entries); label != "Diagnostics console: 1 error, 2 warnings" {
		t.Fatal(label)
	}
	var html bytes.Buffer
	if err := DiagnosticsConsole(entries).Render(context.Background(), &html); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"app.py:3:1", "symbol:app.run", "review-context", "&lt;script&gt;", "Analysis limits"} {
		if !strings.Contains(html.String(), expected) {
			t.Errorf("missing %q", expected)
		}
	}
	if strings.Contains(html.String(), "<script>bad()") {
		t.Fatal("diagnostic rendered as executable HTML")
	}
}

func TestDiagnosticsRetainsDistinctLocations(t *testing.T) {
	browser := application.CodeBrowser{Diagnostics: []ir.Diagnostic{
		{Severity: "warning", Code: "IMPORT", Message: "Cannot resolve dependency", Evidence: []ir.Evidence{{File: "a.py", StartLine: 1}}},
		{Severity: "warning", Code: "IMPORT", Message: "Cannot resolve dependency", Evidence: []ir.Evidence{{File: "b.py", StartLine: 1}}},
	}}
	if entries := codeDiagnostics(browser, CodeFilter{}); len(entries) != 2 {
		t.Fatalf("distinct locations were collapsed: %v", entries)
	}
}
