package main

import (
	"embed"
	"os"

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
	root := cli.New()
	os.Exit(exit.Execute(root, cli.Debug(root)))
}
