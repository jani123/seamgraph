package runtimecheck

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestVersionRequirements(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("version executable stand-in uses a POSIX shell")
	}
	for _, tc := range []struct {
		version      string
		major, minor int
		ok           bool
	}{
		{"go version go1.25.0 linux/amd64", 1, 25, true},
		{"go version go1.24.9 linux/amd64", 1, 25, false},
		{"v22.23.3", 18, 0, true},
		{"v16.0.0", 18, 0, false},
		{"Python 3.10.0", 3, 10, true},
		{"Python 3.9.9", 3, 10, false},
		{"unknown", 18, 0, false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			script := filepath.Join(t.TempDir(), "runtime")
			if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' '"+tc.version+"'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			got := check(context.Background(), spec{name: script, major: tc.major, minor: tc.minor})
			if (got.Error == "") != tc.ok {
				t.Fatalf("got %+v", got)
			}
		})
	}
	missing := check(context.Background(), spec{name: filepath.Join(t.TempDir(), "missing")})
	if missing.Error == "" {
		t.Fatal("missing runtime reported available")
	}
}

func TestLanguageSelection(t *testing.T) {
	names, err := ToolsForLanguages("python,typescript,python")
	if err != nil || len(names) != 2 || names[0] != "python3" || names[1] != "node" {
		t.Fatalf("%v %v", names, err)
	}
	if _, err = ToolsForLanguages("ruby"); err == nil {
		t.Fatal("accepted unsupported language")
	}
}
