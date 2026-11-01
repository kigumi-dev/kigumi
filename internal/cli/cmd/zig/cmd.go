// Package zig implements `kigumi zig`: install, list, remove and locate the
// zig toolchains kigumi manages itself, under the user's data directory.
package zig

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "zig",
		Short: "Manage the zig toolchains kigumi downloads, builds and runs builds with",
	}
	cmd.PersistentFlags().String("zig-home", "", "managed store directory (default: $KIGUMI_ZIG_HOME, or a data directory under the user's home)")
	cmd.AddCommand(installCmd(), listCmd(), removeCmd(), whichCmd())
	return cmd
}

func installCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [version]",
		Short: "Download (or, with --source, build) a zig version into the managed store",
		Args:  cobra.MaximumNArgs(1),
		RunE:  session.RunE(parseOptions, runInstall),
	}
	cmd.Flags().String("source", "", "build from a git url or local checkout, optionally @ref, instead of downloading a release")
	cmd.Flags().String("base-url", "", "override ziglang.org/download/index.json (a mirror, or a test server)")
	return cmd
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the zig versions installed in the managed store",
		Args:  cobra.NoArgs,
		RunE:  session.RunE(parseOptions, runList),
	}
}

func removeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <version>",
		Short: "Remove an installed zig version from the managed store",
		Args:  cobra.ExactArgs(1),
		RunE:  session.RunE(parseOptions, runRemove),
	}
}

func whichCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "which",
		Short: "Print the zig a native build here would use, and why",
		Args:  cobra.NoArgs,
		RunE:  session.RunE(parseOptions, runWhich),
	}
}
