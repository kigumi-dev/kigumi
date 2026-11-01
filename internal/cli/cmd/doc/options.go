package doc

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

type options struct {
	session session.Session
	target  string
	outDir  string
	all     bool
	html    bool
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	opts := options{session: session.Read(cmd), target: args[0]}
	opts.outDir, _ = cmd.Flags().GetString("output")
	opts.all, _ = cmd.Flags().GetBool("all")
	opts.html, _ = cmd.Flags().GetBool("html")
	return opts, nil
}

type examplesOptions struct {
	root   string
	outDir string
	html   bool
}

func parseExamplesOptions(cmd *cobra.Command, args []string) (examplesOptions, error) {
	opts := examplesOptions{root: "examples"}
	if len(args) == 1 {
		opts.root = args[0]
	}
	opts.outDir, _ = cmd.Flags().GetString("output")
	opts.html, _ = cmd.Flags().GetBool("html")
	return opts, nil
}
