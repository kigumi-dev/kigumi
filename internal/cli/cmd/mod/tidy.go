package mod

import (
	"fmt"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/module"
)

// runTidy drops Requires (and their Replace/lock records) nothing imports; it never adds one, since an unknown import has no url to fetch.
func runTidy(io session.IO, opts options) error {
	mf, ok, err := module.ReadModFile(opts.root)
	if err != nil {
		return errors.WrapErr(err, "read manifest")
	}
	if !ok {
		return &exit.UsageError{Err: errors.NewErrf("%s has no %s", opts.root, module.ManifestName)}
	}
	m, err := driver.LoadModule(opts.root, driver.LoadOptions{Test: true, NoDeps: true, Compiler: session.CompilerOptions()})
	if err != nil {
		return errors.WrapErr(err, "load module")
	}
	used := m.ImportedModules(mf)
	unused := map[string]bool{}
	for _, r := range mf.Requires {
		if !used[r.Name] {
			unused[r.Name] = true
		}
	}
	if len(unused) == 0 {
		fmt.Fprintln(io.Msg, "nothing to remove")
		return nil
	}
	lock, err := module.ReadLock(opts.root)
	if err != nil {
		return errors.WrapErr(err, "read lock")
	}
	for name := range unused {
		delete(lock.Entries, name)
	}
	if err := module.RemoveRequires(opts.root, unused); err != nil {
		return errors.WrapErr(err, "edit manifest")
	}
	if err := module.RemoveLocalReplaces(opts.root, unused); err != nil {
		return errors.WrapErr(err, "edit local overrides")
	}
	if err := module.WriteLock(opts.root, lock); err != nil {
		return errors.WrapErr(err, "write lock")
	}
	for name := range unused {
		fmt.Fprintf(io.Msg, "removed %s\n", name)
	}
	return nil
}
