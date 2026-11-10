package mod

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

type options struct {
	root string
	args []string
	url  string
	// update accepts the version changes tidy lists.
	update  bool
	session session.Session
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	opts := options{args: args, session: session.Read(cmd)}
	opts.root, _ = cmd.Flags().GetString("chdir")
	opts.update, _ = cmd.Flags().GetBool("update")
	opts.url, _ = cmd.Flags().GetString("url")
	return opts, nil
}
