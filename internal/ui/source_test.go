package ui

import "testing"

func TestSourceSeparatesNumberingWithoutChangingContent(t *testing.T) {
	numbered := "    1  package main\n    2  \t// <script>unsafe</script>\n    3    indented\n    4  \n"
	want := "package main\n\t// <script>unsafe</script>\n  indented\n"
	if got := sourcePlain(numbered); got != want {
		t.Fatalf("source changed: %q", got)
	}
	if got := sourceNumbers(numbered); got != "1\n2\n3\n4\n" {
		t.Fatal(got)
	}
	if sourceNumbers("") != "" || sourcePlain("") != "" {
		t.Fatal("empty source changed")
	}
}
