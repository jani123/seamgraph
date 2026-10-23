// packagezip creates the Windows archive without requiring an external zip tool.
package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func archive(source, target string) (err error) {
	// #nosec G304 G703 -- Local packaging CLI deliberately accepts caller-selected input and output paths.
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	// #nosec G304 G703 -- The output archive path is selected by the local packaging command, not a remote client.
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
	}()
	writer := zip.NewWriter(out)
	entry, err := writer.Create(filepath.Base(source))
	if err != nil {
		return err
	}
	if _, err = io.Copy(entry, in); err != nil {
		return err
	}
	return writer.Close()
}
func main() {
	if len(os.Args) != 3 {
		_, _ = fmt.Fprintln(os.Stderr, "usage: packagezip input output.zip")
		os.Exit(2)
	}
	if err := archive(os.Args[1], os.Args[2]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
