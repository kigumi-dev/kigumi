// Package zig implements `kigumi zig`: install, list, remove and locate the
// zig toolchains kigumi manages itself, under the user's data directory.
package zig

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/shared"
	"kigumi/internal/driver"
	"kigumi/internal/zigtool/store"
)

// Cmd builds the `kigumi zig` command tree.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "zig",
		Short: "Manage the zig toolchains kigumi downloads, builds and runs builds with",
	}
	cmd.PersistentFlags().String("zig-home", "", "managed store directory (default: $KIGUMI_ZIG_HOME, or a data directory under the user's home)")
	cmd.AddCommand(installCmd(), listCmd(), removeCmd(), whichCmd())
	return cmd
}

// storeRoot resolves the managed store's directory: --zig-home, else shared.ZigStoreRoot's default.
func storeRoot(cmd *cobra.Command) (string, error) {
	if home, _ := cmd.Flags().GetString("zig-home"); home != "" {
		return store.Root(home)
	}
	return shared.ZigStoreRoot()
}

// modulePin is the zig version the module in the working directory pins,
// or "" when there is none (no mod.kg, or an unpinned one).
func modulePin() string {
	mf, _, err := driver.ReadModFile(".")
	if err != nil {
		return ""
	}
	return mf.Zig
}
