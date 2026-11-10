// Package expand implements `kigumi expand`: the generated source the
// checker added to the module, with the derive requests it came from.
package expand

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expand [--format text|json] <module-root>",
		Short: "Print the source the compiler generated (derive methods) and the declarations it came from",
		Args:  cobra.ExactArgs(1),
		RunE:  session.RunE(parseOptions, run),
	}
	cmd.Flags().String("format", "text", "output format: text (default) or json")
	return cmd
}
