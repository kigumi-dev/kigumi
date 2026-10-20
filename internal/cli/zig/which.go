package zig

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/shared"
	"kigumi/internal/zigtool"
)

func whichCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "which",
		Short: "Print the zig a native build here would use, and why",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := storeRoot(cmd)
			if err != nil {
				return err
			}
			want := modulePin()
			r := zigtool.ForModule(shared.CompilerOptions().ToolLookup(), root, shared.ZigPath(), want)
			switch {
			case r.Path == "" && want == "":
				cmd.Println("no zig found, and this module is unpinned (a native build falls back to clang)")
			case r.Path == "":
				cmd.Printf("no zig satisfies this module's pin (%s); run `kigumi zig install` or set KIGUMI_ZIG\n", want)
			case want == "":
				cmd.Printf("%s (%s, unpinned)\n", r.Path, r.Source)
			case r.Mismatch:
				cmd.Printf("%s (%s, reports %s, does not satisfy pin %s)\n", r.Path, r.Source, r.Version, want)
			case r.Err != nil:
				cmd.Printf("%s (%s, version unknown: %s; pin %s)\n", r.Path, r.Source, r.Err, want)
			default:
				cmd.Printf("%s (%s, satisfies pin %s)\n", r.Path, r.Source, want)
			}
			return nil
		},
	}
}
