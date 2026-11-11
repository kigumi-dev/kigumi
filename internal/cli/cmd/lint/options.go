package lint

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

type options struct {
	session session.Session
	target  string
	diag    session.DiagOptions
	fix     bool
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	diag, err := session.ReadDiagOptions(cmd)
	if err != nil {
		return options{}, err
	}
	fix, _ := cmd.Flags().GetBool("fix")
	return options{session: session.Read(cmd), target: args[0], diag: diag, fix: fix}, nil
}
