// Package mod implements `kigumi mod`: create and edit mod.kg, prune unused
// requires, verify the cache against mod.lock.kg and export both as JSON.
package mod

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mod",
		Short: "Manage the module manifest (mod.kg) and lock file",
	}
	cmd.PersistentFlags().StringP("chdir", "C", ".", "module root to operate on")
	cmd.AddCommand(initCmd(), addCmd(), replaceCmd(), tidyCmd(), verifyCmd(), jsonCmd(), vendorCmd(), renameCmd())
	return cmd
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [<module path>]",
		Short: "Create mod.kg naming the module (default: the directory name) and the toolchain version",
		Args:  cobra.MaximumNArgs(1),
		RunE:  session.RunE(parseOptions, runInit),
	}
}

func addCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <module path> <version>",
		Short: "Append a Require to mod.kg (run `kigumi get` to fetch it)",
		Args:  cobra.ExactArgs(2),
		RunE:  session.RunE(parseOptions, runAdd),
	}
	cmd.Flags().String("url", "", "clone URL, when it cannot be derived from the module path")
	return cmd
}

func replaceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "replace <module path> <directory>",
		Short: "Resolve a dependency from a local directory (written to mod.local.kg)",
		Args:  cobra.ExactArgs(2),
		RunE:  session.RunE(parseOptions, runReplace),
	}
}

func tidyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tidy",
		Short: "Remove Requires that no package imports",
		Args:  cobra.NoArgs,
		RunE:  session.RunE(parseOptions, runTidy),
	}
	cmd.Flags().BoolP("update", "u", false, "accept the version changes removing a Require causes; run `kigumi get` afterwards to lock them")
	return cmd
}

func verifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Check that cached dependencies still match mod.lock.kg",
		Args:  cobra.NoArgs,
		RunE:  session.RunE(parseOptions, runVerify),
	}
}

func jsonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "json",
		Short: "Print the manifest and lock file as JSON",
		Args:  cobra.NoArgs,
		RunE:  session.RunE(parseOptions, runJSON),
	}
}

func vendorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vendor",
		Short: "Copy the locked dependencies into vendor/ and write vendor/modules.txt",
		Args:  cobra.NoArgs,
		RunE:  session.RunE(parseOptions, runVendor),
	}
}

func renameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <new module path> | <old module path> <new module path>",
		Short: "Rename this module (with one argument) or a module it names, rewriting every import",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  session.RunE(parseOptions, runRename),
	}
}
