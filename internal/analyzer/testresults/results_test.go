package testresults

import (
	"context"
	"strings"
	"testing"
)

func TestActualFailuresAndUnsupportedArtifacts(t *testing.T) {
	run := Run{Runner: "pytest", Revision: "abc", Fingerprint: "inputs", RunID: "run", ObservedAt: "2026-10-16T00:00:00Z"}
	e, err := Import("junit", []byte(`<testsuite><testcase classname="world" name="read"><failure>wrong result</failure></testcase><testcase classname="world" name="boundary"><skipped/></testcase></testsuite>`), run)
	if err != nil || len(e) != 2 {
		t.Fatal(e, err)
	}
	outcomes := map[string]string{}
	for _, e := range e {
		outcomes[e.TestID] = e.Outcome
	}
	if outcomes["world/read"] != "failed" || outcomes["world/boundary"] != "skipped" {
		t.Fatal(outcomes)
	}
	for _, tc := range []struct{ format, data string }{{"coverage", "{}"}, {"vitest-json", "{}"}, {"go-test-json", "not-json"}, {"junit", "<invalid"}, {"junit", `<testsuite><testcase classname="x" name="same"/><testcase classname="x" name="same"/></testsuite>`}} {
		if _, err := Import(tc.format, []byte(tc.data), run); err == nil {
			t.Fatal("unsupported/malformed/ambiguous artifact accepted", tc)
		}
	}
}

func TestGoRunnerPreservesToolchainSelection(t *testing.T) {
	t.Setenv("GOTOOLCHAIN", "go1.26.10+auto")
	t.Setenv("GOSUMDB", "sum.golang.org")
	t.Setenv("GOPROXY", "https://proxy.golang.org")
	cmd, _, _, err := Command(context.Background(), t.TempDir(), t.TempDir(), "go")
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{}
	for _, entry := range cmd.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		env[key] = value
	}
	if env["GOTOOLCHAIN"] != "go1.26.10+auto" || env["GOSUMDB"] != "sum.golang.org" || env["GOPROXY"] != "off" {
		t.Fatalf("runner environment: GOTOOLCHAIN=%q GOSUMDB=%q GOPROXY=%q", env["GOTOOLCHAIN"], env["GOSUMDB"], env["GOPROXY"])
	}
}
