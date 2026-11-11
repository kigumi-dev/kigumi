// Package lint implements `kigumi lint`: report the standard lint profile's
// warnings and apply its safe fixes.
package lint

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint [--fix] <module-root|entry.kg>",
		Short: "Report the standard lint warnings for a module",
		Long: "Run the standard lint profile over a module: Eq/Hash mismatches, " +
			"redundant modifiers and `@impl`, `Option[Bool] ||`, unused imports " +
			"and the checker's other warnings. Warnings alone exit 0; --deny-warnings " +
			"makes them fail. --fix rewrites the safe cases in place.",
		Args: cobra.ExactArgs(1),
		RunE: session.RunE(parseOptions, run),
	}
	session.AddDiagFlags(cmd)
	cmd.Flags().Bool("fix", false, "rewrite safe lint findings in place")
	return cmd
}
