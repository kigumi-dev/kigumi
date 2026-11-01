package zig

import (
	"fmt"

	"kigumi/internal/cli/session"
	zigtool "kigumi/internal/toolchain/zig"
)

func runWhich(io session.IO, opts options) error {
	root, err := storeRoot(opts.zigHome)
	if err != nil {
		return err
	}
	want := modulePin()
	r := zigtool.ForModule(session.CompilerOptions().ToolLookup(), root, session.ZigPath(), want)
	switch {
	case r.Path == "" && want == "":
		fmt.Fprintln(io.Out, "no zig found, and this module is unpinned (a native build falls back to clang)")
	case r.Path == "":
		fmt.Fprintf(io.Out, "no zig satisfies this module's pin (%s); run `kigumi zig install` or set KIGUMI_ZIG\n", want)
	case want == "":
		fmt.Fprintf(io.Out, "%s (%s, unpinned)\n", r.Path, r.Source)
	case r.Mismatch:
		fmt.Fprintf(io.Out, "%s (%s, reports %s, does not satisfy pin %s)\n", r.Path, r.Source, r.Version, want)
	case r.Err != nil:
		fmt.Fprintf(io.Out, "%s (%s, version unknown: %s; pin %s)\n", r.Path, r.Source, r.Err, want)
	default:
		fmt.Fprintf(io.Out, "%s (%s, satisfies pin %s)\n", r.Path, r.Source, want)
	}
	return nil
}
