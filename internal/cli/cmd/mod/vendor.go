package mod

import (
	"fmt"
	"path/filepath"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/module"
)

func runVendor(io session.IO, opts options) error {
	mods, err := module.VendorTree(session.CompilerOptions(), opts.root)
	if err != nil {
		return errors.WrapErr(err, "vendor dependencies")
	}
	if len(mods) == 0 {
		fmt.Fprintln(io.Err, "no dependencies to vendor")
		return nil
	}
	for _, m := range mods {
		fmt.Fprintf(io.Err, "vendored %s %s %s\n", m.Name, m.Version, m.Hash)
	}
	fmt.Fprintf(io.Err, "wrote %s\n", filepath.Join(opts.root, module.VendorDirName, module.VendorFileName))
	return nil
}
