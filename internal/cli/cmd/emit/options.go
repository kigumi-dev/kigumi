package emit

import (
	"github.com/spf13/cobra"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
)

type schemaOptions struct {
	session     session.Session
	target      string
	attr        string
	allowErrors bool
	out         string
}

func parseSchemaOptions(cmd *cobra.Command, args []string) (schemaOptions, error) {
	opts := schemaOptions{session: session.Read(cmd), target: args[0]}
	opts.attr, _ = cmd.Flags().GetString("attr")
	opts.allowErrors, _ = cmd.Flags().GetBool("allow-errors")
	opts.out, _ = cmd.Flags().GetString("output")
	return opts, nil
}

type headerOptions struct {
	session session.Session
	target  string
	guard   string
	format  string
	out     string
}

func parseHeaderOptions(cmd *cobra.Command, args []string) (headerOptions, error) {
	opts := headerOptions{session: session.Read(cmd), target: args[0]}
	opts.guard, _ = cmd.Flags().GetString("guard")
	opts.format, _ = cmd.Flags().GetString("format")
	if opts.format != "text" && opts.format != "json" {
		return opts, &exit.ValueError{Err: errors.NewErrf("--format: want text or json, got %q", opts.format)}
	}
	opts.out, _ = cmd.Flags().GetString("output")
	return opts, nil
}
