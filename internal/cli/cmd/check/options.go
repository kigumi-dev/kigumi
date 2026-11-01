package check

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

type options struct {
	session session.Session
	target  string
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	return options{session: session.Read(cmd), target: args[0]}, nil
}
