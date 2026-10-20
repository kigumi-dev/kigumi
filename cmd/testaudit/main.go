// Command testaudit prints the B0 test-inventory snapshot as JSON.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"kigumi/internal/testtool"
)

func main() {
	root := flag.String("root", ".", "repository root")
	out := flag.String("out", "", "output file (default: stdout)")
	flag.Parse()

	a, err := testtool.BuildAudit(*root, exec.LookPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "testaudit:", err)
		os.Exit(1)
	}
	b, err := a.JSON()
	if err != nil {
		fmt.Fprintln(os.Stderr, "testaudit:", err)
		os.Exit(1)
	}
	if *out == "" {
		os.Stdout.Write(b)
		return
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "testaudit:", err)
		os.Exit(1)
	}
}
