// Command e2ebench builds the kigumi compiler once, then measures cold
// check, warm check, build/link and run against one module, writing a JSON Lines event per phase.
package main

import (
	"flag"
	"fmt"
	"os"

	"kigumi/internal/testtool"
)

func main() {
	repoRoot := flag.String("repo-root", ".", "repository root (holds go.mod and main.go)")
	module := flag.String("module", "examples/fizzbuzz", "module or example to check/build/run, relative to -repo-root")
	std := flag.String("std", "std", "std root, relative to -repo-root")
	workDir := flag.String("work-dir", "", "scratch directory for the built compiler and artifact (default: a temp dir this run creates and removes)")
	out := flag.String("out", "", "output file for the JSON Lines events (default: stdout)")
	flag.Parse()

	dir := *workDir
	if dir == "" {
		d, err := os.MkdirTemp("", "kigumi-e2ebench-")
		if err != nil {
			fmt.Fprintln(os.Stderr, "e2ebench:", err)
			os.Exit(1)
		}
		defer os.RemoveAll(d)
		dir = d
	}

	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "e2ebench:", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}

	err := testtool.RunE2E(testtool.E2EOptions{RepoRoot: *repoRoot, Module: *module, StdRoot: *std, WorkDir: dir}, w)
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2ebench:", err)
		os.Exit(1)
	}
}
