// Package token implements `kigumi token`: print the token kind of a word.
package token

import (
	"fmt"

	"github.com/spf13/cobra"

	langtoken "kigumi/internal/lang/token"
)

func New() *cobra.Command {
	return &cobra.Command{
		Use:    "token <word>",
		Short:  "Print the token kind of a word",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), langtoken.Lookup(args[0]))
			return nil
		},
	}
}
