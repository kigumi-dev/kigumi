// Package check implements `kigumi check`: type-check a module.
package check

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <module-root|entry.kg>",
		Short: "Type-check a module and print its diagnostics",
		Args:  cobra.ExactArgs(1),
		RunE:  session.RunE(parseOptions, run),
	}
	session.ReleaseFlag(cmd)
	session.AddDiagFlags(cmd)
	return cmd
}
