// Package fmt implements `kigumi fmt`: print, rewrite or check the
// canonical form of source files.
package fmt

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fmt [-w|--check] <file>...",
		Short: "Print the canonical form of source files",
		Args:  cobra.MinimumNArgs(1),
		RunE:  session.RunE(parseOptions, run),
	}
	cmd.Flags().BoolP("write", "w", false, "rewrite the files in place")
	cmd.Flags().Bool("check", false, "exit 1 when a file is not in canonical form")
	return cmd
}
