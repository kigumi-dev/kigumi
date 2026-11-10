package main

import (
	"embed"
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/cli"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
)

// The std stubs ship inside the executable so an installed kigumi works
// outside the repository.
//
//go:embed all:std
var stdFS embed.FS

func main() {
	session.SetEmbeddedStd(stdFS, "std")
	dispatchShell()
	root := cli.New()
	os.Exit(exit.Execute(root, cli.Debug(root)))
}

// dispatchShell makes the same binary usable as a login shell. A kgsh
// symlink with no arguments enters the REPL; a .kg argument keeps the normal
// script runner, and flags are REPL flags.
func dispatchShell() {
	base := filepath.Base(os.Args[0])
	if base != "kgsh" && base != "-kgsh" {
		return
	}
	args := os.Args[1:]
	if len(args) > 0 && strings.HasSuffix(args[0], ".kg") {
		return
	}
	os.Args = append([]string{os.Args[0], "repl"}, args...)
}
