package fmt

import (
	"github.com/spf13/cobra"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
)

type options struct {
	paths        []string
	write, check bool
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	opts := options{paths: args}
	opts.write, _ = cmd.Flags().GetBool("write")
	opts.check, _ = cmd.Flags().GetBool("check")
	if opts.check && opts.write {
		return opts, &exit.UsageError{Err: errors.NewErr("--check and --write are incompatible")}
	}
	return opts, nil
}
