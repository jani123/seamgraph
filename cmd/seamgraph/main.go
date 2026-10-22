package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"seamgraph/internal/application"
	"seamgraph/internal/bootstrap"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"seamgraph/internal/report"
	"seamgraph/internal/runtimecheck"
	"seamgraph/internal/server"
	"seamgraph/internal/verification"
)

var version = "dev"
var commit = "unknown"
var buildDate = "unknown"

func writeJSON(w io.Writer, v any) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
func run(args []string, out, errOut io.Writer) int {
	return runContext(context.Background(), args, out, errOut)
}
func runContext(ctx context.Context, args []string, out, errOut io.Writer) int {
	fail := func(err error) int { fmt.Fprintln(errOut, err); return 2 }
	if len(args) == 0 {
		return fail(fmt.Errorf("usage: seamgraph <init|validate|scan|check|snapshot|inspect|diff|serve|open|doctor|version> [options]"))
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Fprintf(out, "seamgraph %s (commit %s, built %s)\n", version, commit, buildDate)
		return 0
	}
	command := args[0]
	inspectID := ""
	args = args[1:]
	if (command == "inspect" || command == "open") && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		inspectID = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	languages := fs.String("languages", "", "doctor runtime selection: go,typescript,python,git,ssh")
	root := fs.String("root", ".", "project root")
	remoteRef := fs.String("ref", "HEAD", "remote branch, tag or commit for open")
	cacheDir := fs.String("cache-dir", "", "managed repository workspace directory")
	configFile := fs.String("config", "", "local seamgraph.yaml to copy into a new remote workspace")
	prepareOnly := fs.Bool("prepare-only", false, "prepare remote workspace without starting the server")
	allowNetwork := fs.Bool("allow-network", false, "allow container network binding; HTTP Host remains localhost-only")
	listenFile := fs.String("listen-file", "", "write actual bound address for development tooling")
	addr := fs.String("addr", "127.0.0.1:8765", "loopback listen address")
	artifact := fs.String("artifact", "", "test result artifact")
	metadata := fs.String("metadata", "", "test run metadata JSON")
	runner := fs.String("runner", "go", "test runner: go, pytest, vitest")
	format := fs.String("format", "text", "text, json or sarif for check")
	output := fs.String("output", "", "output file")
	asJSON := fs.Bool("json", false, "JSON output")
	base := fs.String("base", "HEAD", "base revision")
	head := fs.String("head", "HEAD", "head revision or WORKTREE for current checkout")
	revisionRef := fs.String("revision", "", "inspect committed revision")
	baseline := fs.String("baseline", "", "JSON array of accepted violation IDs")
	contracts := fs.Bool("contracts", false, "enforce contract verification")
	tests := fs.Bool("tests", false, "inspect test evidence")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		return fail(fmt.Errorf("unexpected positional arguments"))
	}
	if *asJSON {
		*format = "json"
	}
	svc, err := bootstrap.New(*root)
	if err != nil {
		return fail(err)
	}
	if command == "doctor" {
		var names []string
		if *languages != "" {
			var err error
			names, err = runtimecheck.ToolsForLanguages(*languages)
			if err != nil {
				return fail(err)
			}
		} else if _, err := os.Stat(filepath.Join(*root, application.ConfigFilename)); err == nil {
			configured, err := svc.ConfiguredLanguages(*root)
			if err != nil {
				return fail(err)
			}
			names, err = runtimecheck.ToolsForLanguages(strings.Join(configured, ","))
			if err != nil {
				return fail(err)
			}
		}
		tools := runtimecheck.Check(ctx, names...)
		code := 0
		for _, tool := range tools {
			if tool.Error != "" {
				code = 2
			}
		}
		if *asJSON {
			if err := writeJSON(out, tools); err != nil {
				return fail(err)
			}
		} else {
			for _, tool := range tools {
				if tool.Error != "" {
					fmt.Fprintf(out, "MISSING %s: %s (%s)\n", tool.Name, tool.Error, tool.Purpose)
				} else {
					fmt.Fprintf(out, "OK %s: %s (%s)\n", tool.Name, tool.Version, tool.Purpose)
				}
			}
			fmt.Fprintln(out, "TypeScript compiler and Python AST helper are embedded. Project dependencies and test runners are separate.")
		}
		return code
	}
	if command == "open" {
		if inspectID == "" {
			return fail(fmt.Errorf("usage: seamgraph open <git-url> [--ref branch-or-tag] [--config file] [--prepare-only]"))
		}
		var supplied []byte
		if *configFile != "" {
			var err error
			supplied, err = os.ReadFile(*configFile)
			if err != nil {
				return fail(err)
			}
			if err = application.ValidateConfiguration(supplied); err != nil {
				return fail(err)
			}
		}
		fmt.Fprintln(errOut, "Preparing repository workspace…")
		workspace, created, err := svc.Open(ctx, inspectID, *remoteRef, *cacheDir, supplied)
		if err != nil {
			return fail(err)
		}
		fmt.Fprintln(errOut, "Workspace:", workspace.Root)
		fmt.Fprintln(errOut, "Commit:", workspace.Commit)
		*root = workspace.Root
		if created {
			fmt.Fprintln(errOut, "Created exploratory seamgraph.yaml: review directory boundaries and define public seams.")
		}
		svc, err = svc.At(*root)
		if err != nil {
			return fail(err)
		}
		if *prepareOnly {
			if *asJSON {
				if err = writeJSON(out, workspace); err != nil {
					return fail(err)
				}
			} else {
				fmt.Fprintln(out, workspace.Root)
			}
			return 0
		}
		command = "serve"
	}
	if command == "init" {
		if err := svc.Initialize(*root); err != nil {
			return fail(err)
		}
		fmt.Fprintln(out, "Created", filepath.Join(*root, application.ConfigFilename))
		return 0
	}
	if command == "validate" {
		count, err := svc.Validate(*root)
		if err != nil {
			return fail(err)
		}
		fmt.Fprintf(out, "Configuration valid: %d modules (analysis not performed)\n", count)
		return 0
	}
	if command == "fingerprint" {
		rev, fingerprint, err := svc.Identity(ctx, *root)
		if err != nil {
			return fail(err)
		}
		if err = writeJSON(out, map[string]any{"schemaVersion": 1, "revision": rev, "fingerprint": fingerprint}); err != nil {
			return fail(err)
		}
		return 0
	}
	if command == "import-tests" {
		if *artifact == "" || *metadata == "" {
			return fail(fmt.Errorf("import-tests requires --artifact and --metadata"))
		}
		store, err := svc.ImportTests(*root, *format, *artifact, *metadata)
		if err != nil {
			return fail(err)
		}
		if err = writeJSON(out, store); err != nil {
			return fail(err)
		}
		return 0
	}
	if command == "test" {
		store, err := svc.RunTests(ctx, *root, *runner)
		if err != nil {
			return fail(err)
		}
		if err = writeJSON(out, store); err != nil {
			return fail(err)
		}
		return 0
	}
	if command == "serve" {
		rootPath, err := filepath.Abs(*root)
		if err != nil {
			return fail(err)
		}
		svc, err = svc.At(rootPath)
		if err != nil {
			return fail(err)
		}
		app, err := server.New(svc)
		if err != nil {
			return fail(err)
		}
		announce := func(bound string) error {
			if *listenFile != "" {
				if err := os.WriteFile(*listenFile, []byte(bound+"\n"), 0600); err != nil {
					return err
				}
			}
			if *allowNetwork {
				fmt.Fprintln(out, "SeamGraph listening on", bound, "(container binding; open the published localhost port)")
			} else {
				fmt.Fprintln(out, "SeamGraph listening at http://"+bound)
			}
			return nil
		}
		if err = server.Serve(ctx, *addr, app.Handler(), *allowNetwork, announce); err != nil {
			return fail(err)
		}
		return 0
	}
	if command == "diff" {
		r, err := svc.Compare(ctx, *base, *head)
		if err != nil {
			return fail(err)
		}
		if *format == "json" {
			if err = writeJSON(out, r); err != nil {
				return fail(err)
			}
		} else if *format == "text" {
			if r.ConfigurationChanged {
				fmt.Fprintln(out, "modified configuration seamgraph.yaml")
			}
			for _, m := range r.Modules {
				for _, file := range m.Files {
					fmt.Fprintf(out, "%s file %s %s", file.Status, m.ID, file.Path)
					if file.OldPath != "" {
						fmt.Fprintf(out, " (from %s)", file.OldPath)
					}
					fmt.Fprintln(out)
				}
			}
			for _, c := range r.Diff.Changes {
				fmt.Fprintf(out, "%s %s %s\n", c.Status, c.Kind, c.EntityID)
			}
		} else {
			return fail(fmt.Errorf("diff supports text or json"))
		}
		return 0
	}
	if !strings.Contains("|scan|snapshot|check|inspect|", "|"+command+"|") {
		return fail(fmt.Errorf("unknown/unavailable command %q", command))
	}
	session, err := svc.InspectProject(ctx, *root, *revisionRef)
	if err != nil {
		return fail(err)
	}
	defer session.Close()
	s, result := session.Snapshot, session.Check
	verify := func() (verification.Report, error) { return session.Verify(ctx) }
	var file *os.File
	if *output != "" {
		file, err = os.Create(*output)
		if err != nil {
			return fail(err)
		}
		defer file.Close()
		out = file
	}
	switch command {
	case "scan", "snapshot":
		if command == "snapshot" && *output == "" {
			return fail(fmt.Errorf("snapshot requires --output FILE"))
		}
		if command == "snapshot" || *format == "json" {
			b, err := s.JSON()
			if err != nil {
				return fail(err)
			}
			if _, err = out.Write(b); err != nil {
				return fail(err)
			}
		} else if *format == "text" {
			fmt.Fprintf(out, "%d modules, %d seams, %d symbols, %d observed relations\n", len(s.Modules), len(s.Seams), len(s.Symbols), len(s.Relations))
			for _, m := range s.Modules {
				fmt.Fprintf(out, "%s (%s)\n", m.ID, strings.Join(m.SourceRoots, ", "))
			}
			for _, d := range result.Diagnostics {
				fmt.Fprintf(out, "%s %s: %s\n", d.Severity, d.Code, d.Message)
			}
		} else {
			return fail(fmt.Errorf("scan supports text or json"))
		}
		if result.Incomplete {
			return 2
		}
		return 0
	case "check":
		if *baseline != "" {
			b, err := os.ReadFile(*baseline)
			if err != nil {
				return fail(err)
			}
			var ids []string
			if err = json.Unmarshal(b, &ids); err != nil {
				return fail(err)
			}
			result = report.ApplyBaseline(result, ids)
		}
		if *contracts {
			cr, err := verify()
			if err != nil {
				return fail(err)
			}
			if *format != "json" {
				return fail(fmt.Errorf("contract checking currently requires --json"))
			}
			if err = writeJSON(out, map[string]any{"schemaVersion": 1, "architecture": result, "contracts": cr}); err != nil {
				return fail(err)
			}
			return max(result.ExitCode(), cr.ExitCode)
		}

		if err = report.Write(out, result, *format); err != nil {
			return fail(err)
		}
		return result.ExitCode()
	case "inspect":
		if inspectID == "" {
			return fail(fmt.Errorf("inspect requires a stable architecture ID"))
		}
		inspection, err := application.Inspect(s, result, inspectID)
		if err != nil {
			return fail(err)
		}
		entity, related := inspection.Entity, inspection.Relations
		var cr *verification.Report
		if *tests {
			r, err := verify()
			if err != nil {
				return fail(err)
			}
			cr = &r
		}
		if *format == "text" {
			fmt.Fprintln(out, inspectID)
			switch x := entity.(type) {
			case ir.Module:
				fmt.Fprintln(out, "Roots:", strings.Join(x.SourceRoots, ", "))
				for _, id := range x.Seams {
					fmt.Fprintln(out, "Seam:", id)
				}
			case ir.Seam:
				for _, symbol := range s.Symbols {
					for _, id := range x.ContractMembers {
						if symbol.ID == id {
							fmt.Fprintln(out, symbol.ID, symbol.DisplaySignature)
						}
					}
				}
			case ir.Symbol:
				fmt.Fprintln(out, x.DisplaySignature)
			case ir.Relation:
				fmt.Fprintln(out, x.Kind, x.FromID, "->", x.ToID)
			case policy.Violation:
				fmt.Fprintln(out, x.Message, x.Remediation)
			}
			for _, r := range related {
				fmt.Fprintln(out, r.Kind, r.FromID, "->", r.ToID)
				for _, e := range r.Evidence {
					fmt.Fprintf(out, "  %s:%d:%d\n", e.File, e.StartLine, e.StartColumn)
				}
			}
			if cr != nil {
				for _, r := range cr.Results {
					if r.SeamID == inspectID {
						fmt.Fprintf(out, "Contract status: %s; member execution coverage: %d/%d; declared requirement verification: %d/%d\n", r.Status, r.MemberCoverage.Covered, r.MemberCoverage.Total, r.RequirementCoverage.Covered, r.RequirementCoverage.Total)
					}
				}
			}
			if result.Incomplete {
				return 2
			}
			return 0
		}
		if *format != "json" {
			return fail(fmt.Errorf("inspect supports text or json"))
		}
		returnErr := writeJSON(out, map[string]any{"contracts": cr, "schemaVersion": 1, "entity": entity, "relations": related, "diagnostics": result.Diagnostics, "violations": result.Violations})
		if returnErr != nil {
			return fail(returnErr)
		}
		if result.Incomplete {
			return 2
		}
		return 0
	}
	return 0
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(runContext(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
