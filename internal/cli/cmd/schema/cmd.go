// Package schema implements `kigumi schema`: the module's declarations with their metadata attributes as JSON,
// the declaration query generators build on.
package schema

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema [--attr <pkg.name>] [--catalog] [--ir] [--allow-errors] [<module-root>]",
		Short: "List declarations and their metadata attributes as JSON, or the std/exp catalog",
		Args:  cobra.MaximumNArgs(1),
		RunE:  session.RunE(parseOptions, run),
	}
	cmd.Flags().String("attr", "", "keep only declarations carrying this attribute")
	cmd.Flags().Bool("allow-errors", false, "print the declarations even when the module has type errors (generators run before their output exists)")
	cmd.Flags().Bool("ir", false, "print the versioned typed Schema IR instead of the legacy signature-string projection")
	cmd.Flags().Bool("catalog", false, "print the std/exp package catalog as JSON instead of declarations")
	return cmd
}
