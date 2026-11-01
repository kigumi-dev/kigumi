package lsp

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

type options struct{ session session.Session }

func parseOptions(cmd *cobra.Command, _ []string) (options, error) {
	return options{session: session.Read(cmd)}, nil
}
