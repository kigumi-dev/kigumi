package mod

import (
	stderrors "errors"
	"fmt"
	"slices"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/module"
)

// runTidy drops Requires (and their Replace records) nothing imports. It never adds a Require, and it never locks: when the removal changes a selected version it lists the change and stops unless --update, leaving the new entry to `get`, which also shows the native diff.
func runTidy(io session.IO, opts options) error {
	mf, ok, err := module.ReadModFile(opts.root)
	if err != nil {
		return errors.WrapErr(err, "read manifest")
	}
	if !ok {
		return &exit.ValueError{Err: errors.NewErrf("%s has no %s", opts.root, module.ManifestName)}
	}
	m, err := driver.LoadModule(opts.root, driver.LoadOptions{Test: true, NoDeps: true, NoLocal: opts.session.NoLocal || session.LocalOff(), Compiler: session.CompilerOptions()})
	if err != nil {
		return errors.WrapErr(err, "load module")
	}
	used := m.ImportedModules(mf)
	unused := map[string]bool{}
	var unusedList []module.Require
	for _, r := range mf.Requires {
		if !used[r.Key()] {
			unused[r.Key()] = true
			unusedList = append(unusedList, r)
		}
	}
	if len(unused) == 0 {
		fmt.Fprintln(io.Err, "nothing to remove")
		return nil
	}
	lock, err := module.ReadLock(opts.root)
	if err != nil {
		return errors.WrapErr(err, "read lock")
	}
	local, err := opts.session.ReadLocal(opts.root)
	if err != nil {
		return errors.WrapErr(err, "read local overrides")
	}
	if local, err = local.WithVendor(opts.root); err != nil {
		return errors.WrapErr(err, "read vendor tree")
	}
	kept := mf
	kept.Requires = nil
	for _, r := range mf.Requires {
		if !unused[r.Key()] {
			kept.Requires = append(kept.Requires, r)
		}
	}
	// A plain-name Replace applies to every alias of that name; only drop it
	// once no kept Require shares the real name.
	removeLocal := map[string]bool{}
	for key := range unused {
		removeLocal[key] = true
	}
	for _, r := range unusedList {
		if !slices.ContainsFunc(kept.Requires, func(have module.Require) bool { return have.Name == r.Name }) {
			removeLocal[r.Name] = true
		}
	}
	for key := range removeLocal {
		delete(local.Replaces, key)
	}
	// Compare against the lock before dropping the removed Requires' own entries:
	// a Require that only sets a version floor changes the selection when it goes,
	// and that must be visible as a change rather than hidden by the deletion.
	pruned, changes, err := module.PruneLock(session.CompilerOptions(), kept, local, lock)
	var missing *module.MissingError
	switch {
	case stderrors.As(err, &missing):
		fmt.Fprintf(io.Err, "%s; the lock is not checked against the new selection until then\n", err)
		for name := range unused {
			delete(lock.Entries, name)
		}
	case err != nil:
		return errors.WrapErr(err, "reselect versions")
	default:
		lock = pruned
	}
	if len(changes) > 0 {
		for _, c := range changes {
			fmt.Fprintf(io.Err, "selection changes: %s\n", c)
		}
		if !opts.update {
			fmt.Fprintln(io.Err, "nothing was changed; rerun with --update to accept, then run `kigumi get` to lock the new versions")
			return exit.Code(1)
		}
	}
	if err := module.RemoveRequires(opts.root, unused); err != nil {
		return errors.WrapErr(err, "edit manifest")
	}
	if err := module.RemoveLocalReplaces(opts.root, removeLocal); err != nil {
		return errors.WrapErr(err, "edit local overrides")
	}
	if err := module.WriteLock(opts.root, lock); err != nil {
		return errors.WrapErr(err, "write lock")
	}
	for name := range unused {
		fmt.Fprintf(io.Err, "removed %s\n", name)
	}
	return nil
}
