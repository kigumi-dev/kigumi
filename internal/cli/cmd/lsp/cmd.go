// Package lsp implements `kigumi lsp`: the language server on stdio.
package lsp

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	return &cobra.Command{
		Use:   "lsp",
		Short: "Speak the Language Server Protocol on stdin/stdout",
		Args:  cobra.NoArgs,
		RunE:  session.RunE(parseOptions, run),
	}
}
