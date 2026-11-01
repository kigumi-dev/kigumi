package explain

import (
	"github.com/spf13/cobra"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
)

type options struct {
	session      session.Session
	args         []string
	fn           string
	capabilities bool
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	opts := options{session: session.Read(cmd), args: args}
	opts.fn, _ = cmd.Flags().GetString("fn")
	opts.capabilities, _ = cmd.Flags().GetBool("capabilities")
	switch {
	case opts.fn != "" && opts.capabilities:
		return opts, &exit.UsageError{Err: errors.NewErrf("--fn and --capabilities are exclusive")}
	case opts.fn != "" && len(args) != 1:
		return opts, &exit.UsageError{Err: errors.NewErrf("--fn needs exactly one module/entry argument")}
	case opts.capabilities && len(args) != 1:
		return opts, &exit.UsageError{Err: errors.NewErrf("--capabilities needs exactly one module/entry argument")}
	}
	return opts, nil
}
