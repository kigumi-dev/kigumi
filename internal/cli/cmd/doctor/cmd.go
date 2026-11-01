// Package doctor implements `kigumi doctor`: a read-only report of what the CLI resolved and where each value came from.
package doctor

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Report the resolved toolchain, std root, target and cache",
		Args:  cobra.NoArgs,
		RunE:  session.RunE(parseOptions, run),
	}
	cmd.Flags().String("format", "text", "output format: text (default) or json")
	return cmd
}
