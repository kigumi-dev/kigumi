package mod

import "github.com/spf13/cobra"

type options struct {
	root string
	args []string
	url  string
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	opts := options{args: args}
	opts.root, _ = cmd.Flags().GetString("chdir")
	opts.url, _ = cmd.Flags().GetString("url")
	return opts, nil
}
