package zig

import (
	"fmt"
	"sort"

	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/zig/store"
)

func runList(io session.IO, opts options) error {
	root, err := storeRoot(opts.zigHome)
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
		fmt.Fprintln(io.Msg, "no zig versions installed")
	}
	for _, v := range versions {
		m, _ := store.ReadManifest(store.VersionDir(root, v))
		mark := ""
		if want != "" && v == want {
			mark = " (needed by this module)"
		}
		fmt.Fprintf(io.Msg, "%s  %s%s\n", v, m.Obtained, mark)
	}
	if want != "" {
		if _, ok := store.Installed(root, want); !ok {
			fmt.Fprintf(io.Msg, "%s  not installed (this module's pin)\n", want)
		}
	}
	return nil
}
