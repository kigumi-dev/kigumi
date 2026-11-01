// Package run implements `kigumi run`: run the entry file of a module on the VM.
package run

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "run [flags] <module-root|entry.kg> [args...]",
		DisableFlagsInUseLine: true,
		Short:                 "Run the entry file",
		Args:                  cobra.MinimumNArgs(1),
		RunE:                  session.RunE(parseOptions, run),
	}
	cmd.Flags().SetInterspersed(false)
	return cmd
}

// Script is exported for the root's shebang path `kigumi file.kg [args]`, where everything after the entry file is
// script argv and never a swallowed kigumi flag.
func Script(cmd *cobra.Command, args []string) error {
	return session.RunE(scriptOptions, run)(cmd, args)
}
