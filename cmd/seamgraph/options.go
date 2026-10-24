package main

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type commandOptions struct {
	languages    string
	root         string
	remoteRef    string
	cacheDir     string
	configFile   string
	prepareOnly  bool
	allowNetwork bool
	listenFile   string
	addr         string
	artifact     string
	metadata     string
	runner       string
	format       string
	output       string
	asJSON       bool
	base         string
	head         string
	revisionRef  string
	baseline     string
	contracts    bool
	tests        bool
}

func bindOptions(cmd *cobra.Command, opts *commandOptions) {
	fs := cmd.Flags()
	fs.StringVar(&opts.languages, "languages", "", "doctor runtime selection: go,typescript,python,git,ssh")
	fs.StringVar(&opts.root, "root", ".", "project root")
	fs.StringVar(&opts.remoteRef, "ref", "HEAD", "remote branch, tag or commit for open")
	fs.StringVar(&opts.cacheDir, "cache-dir", "", "managed repository workspace directory")
	fs.StringVar(&opts.configFile, "config", "", "local seamgraph.yaml to copy into a new remote workspace")
	fs.BoolVar(&opts.prepareOnly, "prepare-only", false, "prepare remote workspace without starting the server")
	fs.BoolVar(&opts.allowNetwork, "allow-network", false, "allow container network binding; HTTP Host remains localhost-only")
	fs.StringVar(&opts.listenFile, "listen-file", "", "write actual bound address for development tooling")
	fs.StringVar(&opts.addr, "addr", "127.0.0.1:8765", "loopback listen address")
	fs.StringVar(&opts.artifact, "artifact", "", "test result artifact")
	fs.StringVar(&opts.metadata, "metadata", "", "test run metadata JSON")
	fs.StringVar(&opts.runner, "runner", "go", "test runner: go, pytest, vitest")
	fs.StringVar(&opts.format, "format", "text", "text, json or sarif for check")
	fs.StringVar(&opts.output, "output", "", "output file")
	fs.BoolVar(&opts.asJSON, "json", false, "JSON output")
	fs.StringVar(&opts.base, "base", "HEAD", "base revision")
	fs.StringVar(&opts.head, "head", "HEAD", "head revision or WORKTREE for current checkout")
	fs.StringVar(&opts.revisionRef, "revision", "", "inspect committed revision")
	fs.StringVar(&opts.baseline, "baseline", "", "JSON array of accepted violation IDs")
	fs.BoolVar(&opts.contracts, "contracts", false, "enforce contract verification")
	fs.BoolVar(&opts.tests, "tests", false, "inspect test evidence")
}

// Keep previously accepted flags available while showing each command's relevant options.
func focusCommandHelp(cmd *cobra.Command) {
	visible := map[string]string{
		"init": "root", "validate": "root", "fingerprint": "root",
		"scan":         "root revision format json output",
		"snapshot":     "root revision format json output",
		"check":        "root revision format json output baseline contracts tests",
		"inspect":      "root revision format json output contracts tests",
		"diff":         "root base head format json",
		"test":         "root runner",
		"import-tests": "root artifact metadata format",
		"serve":        "root addr allow-network listen-file",
		"open":         "root ref cache-dir config prepare-only addr allow-network listen-file json",
		"doctor":       "root languages json",
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Hidden = !strings.Contains(" "+visible[cmd.Name()]+" ", " "+f.Name+" ")
	})
}
