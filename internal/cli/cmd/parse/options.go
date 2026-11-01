package parse

import "github.com/spf13/cobra"

type options struct{ path string }

func parseOptions(_ *cobra.Command, args []string) (options, error) {
	return options{path: args[0]}, nil
}
