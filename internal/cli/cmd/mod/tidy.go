package mod

import (
	stderrors "errors"
	"fmt"

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
		return &exit.UsageError{Err: errors.NewErrf("%s has no %s", opts.root, module.ManifestName)}
	}
	m, err := driver.LoadModule(opts.root, driver.LoadOptions{Test: true, NoDeps: true, NoLocal: opts.session.NoLocal || session.LocalOff(), Compiler: session.CompilerOptions()})
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
	kept := mf
	kept.Requires = nil
	for _, r := range mf.Requires {
		if !unused[r.Name] {
			kept.Requires = append(kept.Requires, r)
		}
	}
	for name := range unused {
		delete(local.Replaces, name)
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
	if err := module.RemoveLocalReplaces(opts.root, unused); err != nil {
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
