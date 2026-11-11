package mod

import (
	"fmt"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/module"
)

// runRename takes one argument (rename this module) or two (rename a module
// it names), and rewrites the manifest, the local overrides and every import
// that belongs to the old path. The lock is never rewritten: a moved module
// has to be fetched again under its new identity.
func runRename(io session.IO, opts options) error {
	old, next := opts.args[0], ""
	if len(opts.args) == 2 {
		next = opts.args[1]
	} else {
		mf, ok, err := module.ReadModFile(opts.root)
		if err != nil {
			return errors.WrapErr(err, "read manifest")
		}
		if !ok || mf.Name == "" {
			return &exit.ValueError{Err: errors.NewErrf("%s has no module name to rename; give the old and new paths", opts.root)}
		}
		old, next = mf.Name, old
	}
	changes, warnings, err := module.RenameModule(opts.root, old, next)
	if err != nil {
		return &exit.ValueError{Err: err}
	}
	for _, c := range changes {
		fmt.Fprintf(io.Err, "rewrote %s\n", c)
	}
	for _, w := range warnings {
		fmt.Fprintln(io.Err, w)
	}
	fmt.Fprintf(io.Err, "renamed %s to %s\n", old, next)
	return nil
}
