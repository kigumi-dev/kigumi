// Package explain implements `kigumi explain <code>`: the catalog entry
// behind a diagnostic code, the way `rustc --explain` works.
package explain

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"kigumi/internal/cliutil"
	"kigumi/internal/errors"
	"kigumi/internal/sem"
)

func Cmd() *cobra.Command {
	var fn string
	var capabilities bool
	cmd := &cobra.Command{
		Use:   "explain <code>",
		Short: "Describe a diagnostic code such as E100 (no argument lists them all), --fn <pkg.Name> <module> for an operation summary, or --capabilities <module|artifact> for its reason graph",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if fn != "" && capabilities {
				return &cliutil.UsageError{Err: errors.NewErrf("--fn and --capabilities are exclusive")}
			}
			if fn != "" {
				if len(args) != 1 {
					return &cliutil.UsageError{Err: errors.NewErrf("--fn needs exactly one module/entry argument")}
				}
				return explainOp(cmd, fn, args[0])
			}
			if capabilities {
				if len(args) != 1 {
					return &cliutil.UsageError{Err: errors.NewErrf("--capabilities needs exactly one module/entry argument")}
				}
				return explainCapabilities(cmd, args[0])
			}
			out := cmd.OutOrStdout()
			if len(args) == 0 {
				for _, c := range sem.Codes() {
					fmt.Fprintf(out, "%s  %s\n", c.Num, c.Name)
				}
				return nil
			}
			want := strings.ToUpper(args[0])
			for _, c := range sem.Codes() {
				if c.Num != want && c.Name != args[0] {
					continue
				}
				fmt.Fprintf(out, "%s (%s)\n\n  %s\n", c.Num, c.Name, c.Template)
				if c.Help != "" {
					fmt.Fprintf(out, "\n  help: %s\n", c.Help)
				}
				return nil
			}
			return &cliutil.UsageError{Err: errors.NewErrf("unknown diagnostic code %q; `kigumi explain` lists them", args[0])}
		},
	}
	cmd.Flags().StringVar(&fn, "fn", "", "print a function's inferred operation summary instead (pkg/path.Name)")
	cmd.Flags().BoolVar(&capabilities, "capabilities", false, "print a module's or a built artifact's reason graph: every capability its reachable code draws on, whether the target provides it, and the call path that reaches it")
	return cmd
}
