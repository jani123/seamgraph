// packagezip creates the Windows archive without requiring an external zip tool.
package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func archive(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
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
		fmt.Fprintln(os.Stderr, "usage: packagezip input output.zip")
		os.Exit(2)
	}
	if err := archive(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
