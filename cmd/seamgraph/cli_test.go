package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLIHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"help", "check"}, {"check", "--help"}, {"open", "--help"}} {
		var out, diagnostics bytes.Buffer
		if code := run(args, &out, &diagnostics); code != 0 || diagnostics.Len() != 0 {
			t.Fatalf("%v: code=%d stderr=%q", args, code, diagnostics.String())
		}
		for _, want := range []string{"Usage:", "Flags:", "--help"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("%v: missing %q in %s", args, want, out.String())
			}
		}
		if args[0] == "check" && (strings.Contains(out.String(), "--addr") || !strings.Contains(out.String(), "--contracts")) {
			t.Fatalf("check help should show relevant flags: %s", out.String())
		}
	}
}

func TestCLIArgumentValidation(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"open"}, {"open", "one", "two"}, {"inspect", "one", "two"}, {"version", "extra"}, {"check", "--unknown"}} {
		var out, diagnostics bytes.Buffer
		if code := run(args, &out, &diagnostics); code != 2 || diagnostics.Len() == 0 {
			t.Fatalf("%v: code=%d stderr=%q", args, code, diagnostics.String())
		}
	}
}

func TestCLIVersionFlag(t *testing.T) {
	var command, flag, diagnostics bytes.Buffer
	if run([]string{"version"}, &command, &diagnostics) != 0 || run([]string{"--version"}, &flag, &diagnostics) != 0 || command.String() != flag.String() {
		t.Fatalf("version output differs: %q %q %q", command.String(), flag.String(), diagnostics.String())
	}
}

func TestCLIFlagsBeforeInspectID(t *testing.T) {
	var out, diagnostics bytes.Buffer
	code := run([]string{"inspect", "--root", "../../testdata/projects/go", "--json", "seam:world.reader"}, &out, &diagnostics)
	if code != 0 || !strings.Contains(out.String(), "world.reader") {
		t.Fatalf("interspersed flags: code=%d stderr=%q output=%q", code, diagnostics.String(), out.String())
	}
}
