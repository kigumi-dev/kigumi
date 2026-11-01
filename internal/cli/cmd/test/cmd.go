// Package test implements `kigumi test`: run the test blocks of a module on the VM.
package test

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <module-root>",
		Short: "Run the test blocks of a module",
		Args:  cobra.ExactArgs(1),
		RunE:  session.RunE(parseOptions, run),
	}
	cmd.Flags().Bool("list", false, "print every selected test's stable id and file:line, and run nothing")
	cmd.Flags().String("filter", "", "keep only tests whose stable id (<module>/<package>::<name>) contains this substring")
	cmd.Flags().Duration("timeout", 0, "fail a single test that runs longer than this (0: no bound)")
	cmd.Flags().Duration("suite-timeout", 0, "stop starting new tests once the whole run has taken this long (0: no bound)")
	cmd.Flags().Bool("json", false, "stream versioned JSON test events on stdout instead of ok/FAIL/TIME lines")
	return cmd
}
