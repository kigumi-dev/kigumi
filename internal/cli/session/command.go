package session

import (
	"io"

	"github.com/spf13/cobra"
)

// IO is where a command writes: its result to Out, progress and side-effect notes to Err.
type IO struct{ Out, Err io.Writer }

func NewIO(cmd *cobra.Command) IO {
	return IO{Out: cmd.OutOrStdout(), Err: cmd.ErrOrStderr()}
}

// RunE joins a command's option parser and its cobra-free run function into a cobra RunE.
func RunE[O any](parse func(*cobra.Command, []string) (O, error), run func(IO, O) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		opts, err := parse(cmd, args)
		if err != nil {
			return err
		}
		return run(NewIO(cmd), opts)
	}
}
