package schema

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

type options struct {
	session     session.Session
	target      string
	attr        string
	allowErrors bool
	ir          bool
	catalog     bool
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	opts := options{session: session.Read(cmd), target: "."}
	if len(args) > 0 {
		opts.target = args[0]
	}
	opts.attr, _ = cmd.Flags().GetString("attr")
	opts.allowErrors, _ = cmd.Flags().GetBool("allow-errors")
	opts.ir, _ = cmd.Flags().GetBool("ir")
	opts.catalog, _ = cmd.Flags().GetBool("catalog")
	return opts, nil
}
