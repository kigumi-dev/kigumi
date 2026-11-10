package check

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

type options struct {
	session session.Session
	target  string
	diag    session.DiagOptions
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	diag, err := session.ReadDiagOptions(cmd)
	if err != nil {
		return options{}, err
	}
	return options{session: session.Read(cmd), target: args[0], diag: diag}, nil
}
