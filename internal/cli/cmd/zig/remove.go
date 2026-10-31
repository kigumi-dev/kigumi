package zig

import (
	"github.com/spf13/cobra"

	"kigumi/internal/toolchain/zig/store"
)

func removeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <version>",
		Short: "Remove an installed zig version from the managed store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := storeRoot(cmd)
			if err != nil {
				return err
			}
			if err := store.Remove(root, args[0]); err != nil {
				return err
			}
			cmd.Printf("zig %s removed\n", args[0])
			return nil
		},
	}
}
