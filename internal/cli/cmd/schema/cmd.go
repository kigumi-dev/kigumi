// Package schema implements `kigumi schema`: the module's declarations with their metadata attributes as JSON,
// the declaration query generators build on.
package schema

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema [--attr <pkg.name>] [--ir] [--allow-errors] <module-root>",
		Short: "List declarations and their metadata attributes as JSON",
		Args:  cobra.ExactArgs(1),
		RunE:  session.RunE(parseOptions, run),
	}
	cmd.Flags().String("attr", "", "keep only declarations carrying this attribute")
	cmd.Flags().Bool("allow-errors", false, "print the declarations even when the module has type errors (generators run before their output exists)")
	cmd.Flags().Bool("ir", false, "print the versioned typed Schema IR instead of the legacy signature-string projection")
	return cmd
}
