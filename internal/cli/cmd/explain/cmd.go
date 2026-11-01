// Package explain implements `kigumi explain <code>`: the catalog entry
// behind a diagnostic code, the way `rustc --explain` works.
package explain

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain <code>",
		Short: "Describe a diagnostic code such as E100 (no argument lists them all), --fn <pkg.Name> <module> for an operation summary, or --capabilities <module|artifact> for its reason graph",
		Args:  cobra.MaximumNArgs(1),
		RunE:  session.RunE(parseOptions, run),
	}
	cmd.Flags().String("fn", "", "print a function's inferred operation summary instead (pkg/path.Name)")
	cmd.Flags().Bool("capabilities", false, "print a module's or a built artifact's reason graph: every capability its reachable code draws on, whether the target provides it, and the call path that reaches it")
	return cmd
}
