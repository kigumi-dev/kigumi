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
		Long: "Run the test blocks of a module.\n\n" +
			"A test's stable id is <package>::<name>, or <module>/<package>::<name> when the package path does not\n" +
			"already name the module. Tests take no arguments: host.args() is empty.",
		Args: cobra.ExactArgs(1),
		RunE: session.RunE(parseOptions, run),
	}
	cmd.Flags().Bool("list", false, "print every selected test's stable id and file:line, and run nothing")
	cmd.Flags().String("filter", "", "keep only tests whose stable id (<package>::<name>) contains this substring")
	cmd.Flags().Duration("timeout", 0, "report a single test that runs longer than this as TIME (0: no bound); the test is stopped at its next host effect")
	cmd.Flags().Duration("suite-timeout", 0, "stop starting new tests once the whole run has taken this long (0: no bound)")
	cmd.Flags().Bool("json", false, "stream versioned JSON test events on stdout instead of ok/FAIL/TIME lines")
	return cmd
}
