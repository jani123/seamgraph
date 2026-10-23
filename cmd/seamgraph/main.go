package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"seamgraph/internal/bootstrap"
	"strings"
	"syscall"
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

// outputWriter preserves the first output failure across formatted text writes.
type outputWriter struct {
	target io.Writer
	err    error
}

func (w *outputWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.target.Write(p)
	if n != len(p) && err == nil {
		err = io.ErrShortWrite
	}
	w.err = err
	return n, err
}
func runContext(ctx context.Context, args []string, out, errOut io.Writer) (exitCode int) {
	fail := func(err error) int { _, _ = fmt.Fprintln(errOut, err); return 2 }
	trackedOutput := &outputWriter{target: out}
	out = trackedOutput
	defer func() {
		if trackedOutput.err != nil && exitCode == 0 {
			exitCode = fail(trackedOutput.err)
		}
	}()
	if len(args) == 0 {
		return fail(fmt.Errorf("usage: seamgraph <init|validate|scan|check|snapshot|inspect|diff|serve|open|doctor|version> [options]"))
	}
	if args[0] == "version" || args[0] == "--version" {
		_, _ = fmt.Fprintf(out, "seamgraph %s (commit %s, built %s)\n", version, commit, buildDate)
		return 0
	}
	command := args[0]
	inspectID := ""
	args = args[1:]
	if (command == "inspect" || command == "open") && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		inspectID = args[0]
		args = args[1:]
	}
	opts, code := parseOptions(command, args, errOut)
	if opts == nil {
		return code
	}
	svc, err := bootstrap.New(opts.root)
	if err != nil {
		return fail(err)
	}
	c := commandInvocation{ctx: ctx, out: out, errOut: errOut, command: command, inspectID: inspectID, opts: opts, svc: svc, trackedOutput: trackedOutput}
	return c.execute()
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(runContext(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
