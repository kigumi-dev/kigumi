// Package lsp implements `kigumi lsp`: the language server on stdio.
package lsp

import (
	"github.com/spf13/cobra"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/session"
	"kigumi/internal/tooling/lsp"
)

func Cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lsp",
		Short: "Speak the Language Server Protocol on stdin/stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := shared.Target(cmd)
			if err != nil {
				return err
			}
			return errors.WrapErr(lsp.Serve(shared.StdRoot(cmd), target, shared.PackageCache(), shared.CompilerOptions(), shared.UnitCheck()), "language server")
		},
	}
}
