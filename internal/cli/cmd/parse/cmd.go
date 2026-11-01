// Package parse implements `kigumi parse`: dump the syntax tree.
package parse

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	return &cobra.Command{
		Use:   "parse <file>",
		Short: "Dump the syntax tree and parse diagnostics of one file",
		Args:  cobra.ExactArgs(1),
		RunE:  session.RunE(parseOptions, run),
	}
}
