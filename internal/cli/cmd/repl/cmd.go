// Package repl implements `kigumi repl` / kgsh.
package repl

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

// New returns the repl command.
func New() *cobra.Command {
	var opts options
	cmd := &cobra.Command{
		Use:                   "repl [flags]",
		DisableFlagsInUseLine: true,
		Short:                 "Evaluate kigumi source interactively or as kgsh",
		Args:                  cobra.NoArgs,
		RunE:                  session.RunE(parseOptions, run),
	}
	cmd.Flags().BoolVarP(&opts.forceInteractive, "interactive", "i", false, "treat input as interactive even when stdin is not a tty")
	cmd.Flags().BoolVarP(&opts.stdinCells, "stdin", "s", false, "read cells from stdin without a prompt")
	cmd.Flags().StringVarP(&opts.command, "command", "c", "", "evaluate one kigumi source string and exit")
	cmd.Flags().BoolVarP(&opts.login, "login", "l", false, "read /etc/kigumirc and ~/.kigumirc")
	cmd.Flags().BoolVar(&opts.noRC, "no-rc", false, "do not read rc files")
	cmd.Flags().BoolVar(&opts.noHistory, "no-history", false, "do not read or write history")
	return cmd
}
