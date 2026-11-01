// Package get implements "kigumi get": fetch a module or script's dependencies into the cache and lock file, or add and fetch one Require.
package get

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [<module path>[@<version>]] [<module-root>|<script.kg>]",
		Short: "Fetch dependencies with git and write the lock file; with a module path, add it first",
		Args:  cobra.MaximumNArgs(2),
		RunE:  session.RunE(parseOptions, run),
	}
	cmd.Flags().BoolP("update", "u", false, "accept a tag that moved since the lock file was written")
	cmd.Flags().Bool("lock", false, "for a script, write <script>.lock.kg next to it")
	cmd.Flags().String("url", "", "clone URL of the module being added, when it cannot be derived from its name")
	cmd.Flags().Bool("diff-json", false, "print the native consent diff (added/removed Source and Link declarations) as JSON on stdout, moving the summary lines to stderr")
	return cmd
}
