package zig

import "github.com/spf13/cobra"

type options struct {
	zigHome string
	args    []string
	source  string
	baseURL string
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	opts := options{args: args}
	opts.zigHome, _ = cmd.Flags().GetString("zig-home")
	opts.source, _ = cmd.Flags().GetString("source")
	opts.baseURL, _ = cmd.Flags().GetString("base-url")
	return opts, nil
}
