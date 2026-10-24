package main

import (
	"fmt"
	"io"
	"seamgraph/internal/bootstrap"

	"github.com/spf13/cobra"
)

func newRootCommand(out, errOut io.Writer, tracked *outputWriter, code *int) *cobra.Command {
	root := &cobra.Command{
		Use: "seamgraph", Short: "Explore and verify software architecture",
		SilenceUsage: true, SilenceErrors: true,
		Version: fmt.Sprintf("%s (commit %s, built %s)", version, commit, buildDate),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cmd.Help(); err != nil {
				return err
			}
			*code = 2
			return nil
		},
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetVersionTemplate("seamgraph {{.Version}}\n")
	for _, spec := range commandSpecs() {
		root.AddCommand(newProjectCommand(spec, tracked, code))
	}
	root.AddCommand(&cobra.Command{
		Use: "version", Short: "Print version and build information", Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) { cmd.Println("seamgraph " + root.Version) },
	})
	return root
}

type commandSpec struct{ use, short string }

func commandSpecs() []commandSpec {
	return []commandSpec{
		{"init", "Create a project configuration"},
		{"validate", "Validate the project configuration"},
		{"scan", "Analyze the project architecture"},
		{"check", "Check architecture rules and contracts"},
		{"snapshot", "Export an architecture snapshot"},
		{"inspect [id]", "Inspect a module, seam, or member"},
		{"diff", "Compare architecture between revisions"},
		{"fingerprint", "Print the project revision and fingerprint"},
		{"test", "Run tests and collect verification evidence"},
		{"import-tests", "Import test results and run metadata"},
		{"serve", "Start the architecture explorer"},
		{"open <git-url>", "Prepare a remote repository and start the explorer"},
		{"doctor", "Check required language runtimes and tools"},
	}
}

func newProjectCommand(spec commandSpec, tracked *outputWriter, code *int) *cobra.Command {
	opts := &commandOptions{}
	cmd := &cobra.Command{Use: spec.use, Short: spec.short, Args: cobra.NoArgs}
	switch cmd.Name() {
	case "inspect":
		cmd.Args = cobra.MaximumNArgs(1)
	case "open":
		cmd.Args = cobra.ExactArgs(1)
	}
	bindOptions(cmd, opts)
	focusCommandHelp(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if opts.asJSON {
			opts.format = "json"
		}
		svc, err := bootstrap.New(opts.root)
		if err != nil {
			return err
		}
		c := commandInvocation{ctx: cmd.Context(), out: cmd.OutOrStdout(), errOut: cmd.ErrOrStderr(),
			command: cmd.Name(), opts: opts, svc: svc, trackedOutput: tracked}
		if len(args) > 0 {
			c.inspectID = args[0]
		}
		*code = c.execute()
		return nil
	}
	return cmd
}
