package doctor

import (
	"github.com/spf13/cobra"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
)

type options struct {
	session session.Session
	format  string
}

func parseOptions(cmd *cobra.Command, _ []string) (options, error) {
	opts := options{session: session.Read(cmd)}
	opts.format, _ = cmd.Flags().GetString("format")
	if opts.format != "text" && opts.format != "json" {
		return opts, &exit.UsageError{Err: errors.NewErrf("--format: want text or json, got %q", opts.format)}
	}
	return opts, nil
}
