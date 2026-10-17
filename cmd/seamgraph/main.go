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
	"runtime"
	"strings"
	"syscall"

	"seamgraph/internal/analyzer/registry"
	"seamgraph/internal/config"
	"seamgraph/internal/gitdiff"
	"seamgraph/internal/ir"
	"seamgraph/internal/policy"
	"seamgraph/internal/report"
	"seamgraph/internal/repository"
	"seamgraph/internal/revision"
	"seamgraph/internal/runtimecheck"
	"seamgraph/internal/server"
	"seamgraph/internal/verification"
)

var version = "dev"
var commit = "unknown"
var buildDate = "unknown"

const starter = `# Declare literal directory roots; adjust before scanning.
version: 1
modules:
  app:
    language: go
    paths: [internal/app]
policies:
  default: deny
  unresolved: error
`

func analyze(ctx context.Context, root string) (ir.Snapshot, policy.Policy, error) {
	s, c, err := registry.Scan(ctx, root)
	if err != nil {
		return s, policy.Policy{}, err
	}
	p, err := c.Policy()
	return s, p, err
}
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
	addr := fs.String("addr", "127.0.0.1:8765", "loopback listen address")
	artifact := fs.String("artifact", "", "test result artifact")
	metadata := fs.String("metadata", "", "test run metadata JSON")
	runner := fs.String("runner", "go", "test runner: go, pytest, vitest")
	format := fs.String("format", "text", "text, json or sarif for check")
	output := fs.String("output", "", "output file")
	asJSON := fs.Bool("json", false, "JSON output")
	base := fs.String("base", "HEAD", "base revision")
	head := fs.String("head", "HEAD", "head revision")
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
	if command == "doctor" {
		var names []string
		if *languages != "" {
			var err error
			names, err = runtimecheck.ToolsForLanguages(*languages)
			if err != nil {
				return fail(err)
			}
		} else if _, err := os.Stat(filepath.Join(*root, config.Filename)); err == nil {
			doc, err := config.Load(*root)
			if err != nil {
				return fail(err)
			}
			var configured []string
			for _, module := range doc.Config.Modules {
				configured = append(configured, module.Language)
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
			if _, err = config.Parse(supplied); err != nil {
				return fail(err)
			}
		}
		fmt.Fprintln(errOut, "Preparing repository workspace…")
		workspace, err := repository.Open(ctx, inspectID, *remoteRef, *cacheDir)
		if err != nil {
			return fail(err)
		}
		fmt.Fprintln(errOut, "Workspace:", workspace.Root)
		fmt.Fprintln(errOut, "Commit:", workspace.Commit)
		*root = workspace.Root
		target := filepath.Join(*root, config.Filename)
		if supplied != nil {
			if err = os.WriteFile(target, supplied, 0644); err != nil {
				return fail(err)
			}
		} else if _, err = os.Stat(target); os.IsNotExist(err) {
			draft, e := config.Discover(*root)
			if e != nil {
				return fail(fmt.Errorf("workspace retained at %s; cannot draft architecture: %w", *root, e))
			}
			if err = os.WriteFile(target, draft, 0644); err != nil {
				return fail(err)
			}
			fmt.Fprintln(errOut, "Created exploratory seamgraph.yaml: review directory boundaries and define public seams.")
		} else if err != nil {
			return fail(err)
		}
		if _, err = config.Load(*root); err != nil {
			return fail(fmt.Errorf("workspace retained at %s: %w", *root, err))
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
		p := filepath.Join(*root, config.Filename)
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return fail(err)
		}
		_, err = io.WriteString(f, starter)
		closeErr := f.Close()
		if err != nil {
			return fail(err)
		}
		if closeErr != nil {
			return fail(closeErr)
		}
		fmt.Fprintln(out, "Created", p)
		return 0
	}
	if command == "validate" {
		d, err := config.Load(*root)
		if err != nil {
			return fail(err)
		}
		fmt.Fprintf(out, "Configuration valid: %d modules (analysis not performed)\n", len(d.Config.Modules))
		return 0
	}
	if command == "fingerprint" {
		rev, fingerprint, err := revision.Identity(ctx, *root)
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
		store, err := importTests(*root, *format, *artifact, *metadata)
		if err != nil {
			return fail(err)
		}
		if err = writeJSON(out, store); err != nil {
			return fail(err)
		}
		return 0
	}
	if command == "test" {
		store, err := runTests(ctx, *root, *runner)
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
		load := func(ctx context.Context) (server.Model, error) {
			s, p, err := analyze(ctx, rootPath)
			if err != nil {
				return server.Model{}, err
			}
			r, err := policy.Evaluate(s, p)
			if err != nil {
				return server.Model{}, err
			}
			cr, err := contractReport(ctx, rootPath, s)
			return server.Model{Snapshot: s, Check: r, Contracts: cr, Policy: p}, err
		}
		cache := &gitdiff.Cache{Context: runtime.Version() + "|" + strings.Join(os.Environ(), "\x00")}
		app, err := server.New(rootPath, load, func(ctx context.Context, base, head string) (gitdiff.Result, error) {
			return cache.Compare(ctx, rootPath, base, head, analyze, revisionContracts(rootPath))
		})
		if err != nil {
			return fail(err)
		}
		if *allowNetwork {
			fmt.Fprintln(out, "SeamGraph listening on", *addr, "(container binding; open the published localhost port)")
		} else {
			fmt.Fprintln(out, "SeamGraph listening at http://"+*addr)
		}
		if err = server.Serve(ctx, *addr, app.Handler(), *allowNetwork); err != nil {
			return fail(err)
		}
		return 0
	}
	if command == "diff" {
		r, err := gitdiff.Compare(ctx, *root, *base, *head, analyze, revisionContracts(*root))
		if err != nil {
			return fail(err)
		}
		if *format == "json" {
			if err = writeJSON(out, r); err != nil {
				return fail(err)
			}
		} else if *format == "text" {
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
	storeRoot := *root
	selectedCommit := ""
	if *revisionRef != "" {
		dir, commit, cleanup, err := gitdiff.Extract(ctx, *root, *revisionRef)
		if err != nil {
			return fail(err)
		}
		defer cleanup()
		selectedCommit = commit
		*root = dir
	}
	s, p, err := analyze(ctx, *root)
	if err != nil {
		return fail(err)
	}
	result, err := policy.Evaluate(s, p)
	if err != nil {
		return fail(err)
	}
	verify := func() (verification.Report, error) {
		if selectedCommit != "" {
			return revisionContracts(storeRoot)(ctx, *root, s, selectedCommit)
		}
		return contractReport(ctx, *root, s)
	}
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
		var entity any
		ids := []string{}
		for _, m := range s.Modules {
			ids = append(ids, m.ID)
			if m.ID == inspectID {
				entity = m
			}
		}
		for _, x := range s.Seams {
			ids = append(ids, x.ID)
			if x.ID == inspectID {
				entity = x
			}
		}
		for _, x := range s.Symbols {
			ids = append(ids, x.ID)
			if x.ID == inspectID {
				entity = x
			}
		}
		for _, r := range s.Relations {
			ids = append(ids, r.ID)
			if r.ID == inspectID {
				entity = r
			}
		}
		for _, v := range result.Violations {
			ids = append(ids, v.ID)
			if v.ID == inspectID {
				entity = v
			}
		}
		if entity == nil {
			if len(ids) > 12 {
				ids = ids[:12]
			}
			return fail(fmt.Errorf("unknown ID %q; candidate IDs: %s", inspectID, strings.Join(ids, ", ")))
		}
		related := []ir.Relation{}
		owner := ir.Owners(s)[inspectID]
		members := map[string]bool{inspectID: true}
		for _, seam := range s.Seams {
			if seam.ID == inspectID {
				for _, m := range seam.ContractMembers {
					members[m] = true
				}
			}
		}
		for _, r := range s.Relations {
			if members[r.FromID] || members[r.ToID] || (strings.HasPrefix(inspectID, "module:") && (ir.Owners(s)[r.FromID] == owner || ir.Owners(s)[r.ToID] == owner)) {
				related = append(related, r)
			}
		}
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
