package zig

import (
	"fmt"

	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/zig/store"
)

func runRemove(io session.IO, opts options) error {
	root, err := storeRoot(opts.zigHome)
	if err != nil {
		return err
	}
	if err := store.Remove(root, opts.args[0]); err != nil {
		return err
	}
	fmt.Fprintf(io.Msg, "zig %s removed\n", opts.args[0])
	return nil
}
