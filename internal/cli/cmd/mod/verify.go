package mod

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/module"
)

func runVerify(io session.IO, opts options) error {
	lock, err := module.ReadLock(opts.root)
	if err != nil {
		return errors.WrapErr(err, "read lock")
	}
	bad := 0
	local, err := opts.session.ReadLocal(opts.root)
	if err != nil {
		return errors.WrapErr(err, "read local overrides")
	}
	if local, err = local.WithVendor(opts.root); err != nil {
		return errors.WrapErr(err, "read vendor tree")
	}
	if mf, ok, err := module.ReadModFile(opts.root); err != nil {
		return errors.WrapErr(err, "read manifest")
	} else if ok {
		problems, err := module.LockDisagreements(session.CompilerOptions(), mf, local, lock)
		if err != nil {
			return errors.WrapErr(err, "select versions")
		}
		for _, p := range problems {
			bad++
			fmt.Fprintln(io.Out, p)
		}
	}
	for _, key := range sortedNames(lock) {
		e := lock.Entries[key]
		name := e.Name
		if name == "" {
			name = key
		}
		label := key
		if name != key {
			label = key + " (" + name + ")"
		}
		if _, replaced := local.Replacement(e.Require()); replaced {
			// A Replace is not locked: `mod vendor` refuses to copy one, and
			// `get` never fetched it.
			continue
		}
		if vendored, ok := local.Vendored[name]; ok && local.Vendor != "" {
			dir := filepath.Join(local.Vendor, filepath.FromSlash(name))
			switch {
			case vendored.Version != e.Version:
				bad++
				fmt.Fprintf(io.Out, "%s %s: vendor has %s\n", label, e.Version, vendored.Version)
			case !dirExists(dir):
				bad++
				fmt.Fprintf(io.Out, "%s %s: missing from %s; run `kigumi mod vendor`\n", label, e.Version, local.Vendor)
			default:
				hash, err := module.ContentTreeHash(dir)
				if err != nil {
					return errors.WrapErr(err, "hash vendored module")
				}
				if hash == vendored.Hash {
					fmt.Fprintf(io.Out, "%s %s: ok (vendored)\n", label, e.Version)
				} else {
					bad++
					fmt.Fprintf(io.Out, "%s %s: MISMATCH in %s\n", label, e.Version, dir)
				}
			}
			continue
		}
		dir, err := module.CacheDir(session.CompilerOptions(), module.Require{Name: name, Version: e.Version, Alias: e.Alias})
		if err != nil {
			return err
		}
		if _, err := os.Stat(dir); err != nil {
			bad++
			fmt.Fprintf(io.Out, "%s %s: not fetched\n", label, e.Version)
			continue
		}
		if !module.CacheReady(dir) {
			bad++
			fmt.Fprintf(io.Out, "%s %s: incomplete cache entry in %s (missing origin marker or manifest)\n", label, e.Version, dir)
			continue
		}
		if err := module.CheckCacheTree(dir); err != nil {
			bad++
			fmt.Fprintf(io.Out, "%s %s: %s\n", label, e.Version, err)
			continue
		}
		hash, err := module.TreeHash(dir)
		if err != nil {
			return errors.WrapErr(err, "hash dependency")
		}
		if hash == e.Hash {
			fmt.Fprintf(io.Out, "%s %s: ok\n", label, e.Version)
		} else {
			bad++
			fmt.Fprintf(io.Out, "%s %s: MISMATCH in %s\n", label, e.Version, dir)
		}
	}
	if bad > 0 {
		return exit.Code(1)
	}
	return nil
}

func dirExists(dir string) bool {
	st, err := os.Stat(dir)
	return err == nil && st.IsDir()
}

func sortedNames(lock module.Lock) []string {
	names := make([]string, 0, len(lock.Entries))
	for n := range lock.Entries {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
