package zig

import (
	"sort"

	"github.com/spf13/cobra"

	"kigumi/internal/zigtool/store"
)

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the zig versions installed in the managed store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := storeRoot(cmd)
			if err != nil {
				return err
			}
			versions, err := store.List(root)
			if err != nil {
				return err
			}
			sort.Strings(versions)
			want := modulePin()
			if len(versions) == 0 {
				cmd.Println("no zig versions installed")
			}
			for _, v := range versions {
				m, _ := store.ReadManifest(store.VersionDir(root, v))
				mark := ""
				if want != "" && v == want {
					mark = " (needed by this module)"
				}
				cmd.Printf("%s  %s%s\n", v, m.Obtained, mark)
			}
			if want != "" {
				if _, ok := store.Installed(root, want); !ok {
					cmd.Printf("%s  not installed (this module's pin)\n", want)
				}
			}
			return nil
		},
	}
}
