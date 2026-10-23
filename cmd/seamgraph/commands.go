package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"seamgraph/internal/application"
	"seamgraph/internal/bootstrap"
	"seamgraph/internal/runtimecheck"
	"seamgraph/internal/server"
	"seamgraph/internal/verification"
	"strings"
)

type commandInvocation struct {
	ctx                context.Context
	out, errOut        io.Writer
	command, inspectID string
	opts               *commandOptions
	svc                commandService
	trackedOutput      *outputWriter
}

func (c *commandInvocation) fail(err error) int { _, _ = fmt.Fprintln(c.errOut, err); return 2 }

func (c *commandInvocation) execute() (exitCode int) {
	switch c.command {
	case "doctor":
		return c.runDoctor()
	case "open":
		return c.runOpen()
	case "init":
		return c.runInit()
	case "validate":
		return c.runValidate()
	case "fingerprint":
		return c.runFingerprint()
	case "import-tests":
		return c.runImportTests()
	case "test":
		return c.runTest()
	case "serve":
		return c.runServe()
	case "diff":
		return c.runDiff()
	}
	if !strings.Contains("|scan|snapshot|check|inspect|", "|"+c.command+"|") {
		return c.fail(fmt.Errorf("unknown/unavailable command %q", c.command))
	}
	session, err := c.svc.InspectProject(c.ctx, c.opts.root, c.opts.revisionRef)
	if err != nil {
		return c.fail(err)
	}
	defer session.Close()
	var file *os.File
	if c.opts.output != "" {
		// #nosec G703 -- This local CLI operation intentionally uses the path explicitly selected by the user.
		file, err = os.Create(c.opts.output)
		if err != nil {
			return c.fail(err)
		}
		defer func() {
			if err := file.Close(); err != nil && exitCode == 0 {
				exitCode = c.fail(err)
			}
		}()
		c.trackedOutput.target = file
	}
	switch c.command {
	case "scan", "snapshot":
		return c.runScan(session)
	case "check":
		return c.runCheck(session)
	case "inspect":
		return c.runInspect(session)
	}
	return 0
}
func (c *commandInvocation) runDoctor() int {
	var names []string
	if c.opts.languages != "" {
		var err error
		names, err = runtimecheck.ToolsForLanguages(c.opts.languages)
		if err != nil {
			return c.fail(err)
		}
		// #nosec G703 -- This local CLI operation intentionally uses the path explicitly selected by the user.
	} else if _, err := os.Stat(filepath.Join(c.opts.root, application.ConfigFilename)); err == nil {
		configured, err := c.svc.ConfiguredLanguages(c.opts.root)
		if err != nil {
			return c.fail(err)
		}
		names, err = runtimecheck.ToolsForLanguages(strings.Join(configured, ","))
		if err != nil {
			return c.fail(err)
		}
	}
	tools := runtimecheck.Check(c.ctx, names...)
	code := 0
	for _, tool := range tools {
		if tool.Error != "" {
			code = 2
		}
	}
	if c.opts.asJSON {
		if err := writeJSON(c.out, tools); err != nil {
			return c.fail(err)
		}
	} else {
		for _, tool := range tools {
			if tool.Error != "" {
				_, _ = fmt.Fprintf(c.out, "MISSING %s: %s (%s)\n", tool.Name, tool.Error, tool.Purpose)
			} else {
				_, _ = fmt.Fprintf(c.out, "OK %s: %s (%s)\n", tool.Name, tool.Version, tool.Purpose)
			}
		}
		_, _ = fmt.Fprintln(c.out, "TypeScript compiler and Python AST helper are embedded. Project dependencies and test runners are separate.")
	}
	return code
}

func (c *commandInvocation) runOpen() int {
	var err error
	if c.inspectID == "" {
		return c.fail(fmt.Errorf("usage: seamgraph open <git-url> [--ref branch-or-tag] [--config file] [--prepare-only]"))
	}
	var supplied []byte
	if c.opts.configFile != "" {
		var err error
		// #nosec G703 -- This local CLI operation intentionally uses the path explicitly selected by the user.
		supplied, err = os.ReadFile(c.opts.configFile)
		if err != nil {
			return c.fail(err)
		}
		if err = application.ValidateConfiguration(supplied); err != nil {
			return c.fail(err)
		}
	}
	_, _ = fmt.Fprintln(c.errOut, "Preparing repository workspace…")
	workspace, created, err := c.svc.Open(c.ctx, c.inspectID, c.opts.remoteRef, c.opts.cacheDir, supplied)
	if err != nil {
		return c.fail(err)
	}
	_, _ = fmt.Fprintln(c.errOut, "Workspace:", workspace.Root)
	_, _ = fmt.Fprintln(c.errOut, "Commit:", workspace.Commit)
	c.opts.root = workspace.Root
	if created {
		_, _ = fmt.Fprintln(c.errOut, "Created exploratory seamgraph.yaml: review directory boundaries and define public seams.")
	}
	c.svc, err = bootstrap.New(c.opts.root)
	if err != nil {
		return c.fail(err)
	}
	if c.opts.prepareOnly {
		if c.opts.asJSON {
			if err = writeJSON(c.out, workspace); err != nil {
				return c.fail(err)
			}
		} else {
			_, _ = fmt.Fprintln(c.out, workspace.Root)
		}
		return 0
	}
	c.command = "serve"
	return c.execute()

}

func (c *commandInvocation) runInit() int {
	if err := c.svc.Initialize(c.opts.root); err != nil {
		return c.fail(err)
	}
	_, _ = fmt.Fprintln(c.out, "Created", filepath.Join(c.opts.root, application.ConfigFilename))
	return 0
}

func (c *commandInvocation) runValidate() int {
	count, err := c.svc.Validate(c.opts.root)
	if err != nil {
		return c.fail(err)
	}
	_, _ = fmt.Fprintf(c.out, "Configuration valid: %d modules (analysis not performed)\n", count)
	return 0
}

func (c *commandInvocation) runFingerprint() int {
	rev, fingerprint, err := c.svc.Identity(c.ctx, c.opts.root)
	if err != nil {
		return c.fail(err)
	}
	if err = writeJSON(c.out, map[string]any{"schemaVersion": 1, "revision": rev, "fingerprint": fingerprint}); err != nil {
		return c.fail(err)
	}
	return 0
}

func (c *commandInvocation) runImportTests() int {
	if c.opts.artifact == "" || c.opts.metadata == "" {
		return c.fail(fmt.Errorf("import-tests requires --artifact and --metadata"))
	}
	store, err := c.svc.ImportTests(c.opts.root, c.opts.format, c.opts.artifact, c.opts.metadata)
	if err != nil {
		return c.fail(err)
	}
	if err = writeJSON(c.out, store); err != nil {
		return c.fail(err)
	}
	return 0
}

func (c *commandInvocation) runTest() int {
	store, err := c.svc.RunTests(c.ctx, c.opts.root, c.opts.runner)
	if err != nil {
		return c.fail(err)
	}
	if err = writeJSON(c.out, store); err != nil {
		return c.fail(err)
	}
	return 0
}

func (c *commandInvocation) runServe() int {
	var err error
	rootPath, err := filepath.Abs(c.opts.root)
	if err != nil {
		return c.fail(err)
	}
	c.svc, err = bootstrap.New(rootPath)
	if err != nil {
		return c.fail(err)
	}
	app, err := server.New(c.svc)
	if err != nil {
		return c.fail(err)
	}
	announce := func(bound string) error {
		if c.opts.listenFile != "" {
			// #nosec G703 -- This local CLI operation intentionally uses the path explicitly selected by the user.
			if err := os.WriteFile(c.opts.listenFile, []byte(bound+"\n"), 0600); err != nil {
				return err
			}
		}
		if c.opts.allowNetwork {
			_, _ = fmt.Fprintln(c.out, "SeamGraph listening on", bound, "(container binding; open the published localhost port)")
		} else {
			_, _ = fmt.Fprintln(c.out, "SeamGraph listening at http://"+bound)
		}
		return nil
	}
	if err = server.Serve(c.ctx, c.opts.addr, app.Handler(), c.opts.allowNetwork, announce); err != nil {
		return c.fail(err)
	}
	return 0
}

func (c *commandInvocation) runDiff() int {
	r, err := c.svc.Compare(c.ctx, c.opts.base, c.opts.head)
	if err != nil {
		return c.fail(err)
	}
	if c.opts.format == "json" {
		if err = writeJSON(c.out, r); err != nil {
			return c.fail(err)
		}
	} else if c.opts.format == "text" {
		if r.ConfigurationChanged {
			_, _ = fmt.Fprintln(c.out, "modified configuration seamgraph.yaml")
		}
		for _, m := range r.Modules {
			for _, file := range m.Files {
				_, _ = fmt.Fprintf(c.out, "%s file %s %s", file.Status, m.ID, file.Path)
				if file.OldPath != "" {
					_, _ = fmt.Fprintf(c.out, " (from %s)", file.OldPath)
				}
				_, _ = fmt.Fprintln(c.out)
			}
		}
		for _, change := range r.Diff.Changes {
			_, _ = fmt.Fprintf(c.out, "%s %s %s\n", change.Status, change.Kind, change.EntityID)
		}
	} else {
		return c.fail(fmt.Errorf("diff supports text or json"))
	}
	return 0
}

// commandService exposes the use cases consumed by the CLI. Concrete service
// construction stays behind bootstrap.New.
type commandService interface {
	server.Explorer
	ConfiguredLanguages(string) ([]string, error)
	Initialize(string) error
	Validate(string) (int, error)
	Identity(context.Context, string) (string, string, error)
	Open(context.Context, string, string, string, []byte) (application.Workspace, bool, error)
	RunTests(context.Context, string, string) (verification.Store, error)
	ImportTests(string, string, string, string) (verification.Store, error)
	InspectProject(context.Context, string, string) (*application.InspectionSession, error)
}
