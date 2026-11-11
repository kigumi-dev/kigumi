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
	for _, name := range sortedNames(lock) {
		e := lock.Entries[name]
		if _, replaced := local.Replaces[name]; replaced {
			// A Replace is not locked: `mod vendor` refuses to copy one, and
			// `get` never fetched it.
			continue
		}
		if vendored, ok := local.Vendored[name]; ok && local.Vendor != "" {
			dir := filepath.Join(local.Vendor, filepath.FromSlash(name))
			switch {
			case vendored.Version != e.Version:
				bad++
				fmt.Fprintf(io.Out, "%s %s: vendor has %s\n", name, e.Version, vendored.Version)
			case !dirExists(dir):
				bad++
				fmt.Fprintf(io.Out, "%s %s: missing from %s; run `kigumi mod vendor`\n", name, e.Version, local.Vendor)
			default:
				hash, err := module.ContentTreeHash(dir)
				if err != nil {
					return errors.WrapErr(err, "hash vendored module")
				}
				if hash == vendored.Hash {
					fmt.Fprintf(io.Out, "%s %s: ok (vendored)\n", name, e.Version)
				} else {
					bad++
					fmt.Fprintf(io.Out, "%s %s: MISMATCH in %s\n", name, e.Version, dir)
				}
			}
			continue
		}
		dir, err := module.CacheDir(session.CompilerOptions(), module.Require{Name: name, Version: e.Version})
		if err != nil {
			return err
		}
		if _, err := os.Stat(dir); err != nil {
			bad++
			fmt.Fprintf(io.Out, "%s %s: not fetched\n", name, e.Version)
			continue
		}
		if !module.CacheReady(dir) {
			bad++
			fmt.Fprintf(io.Out, "%s %s: incomplete cache entry in %s (missing origin marker or manifest)\n", name, e.Version, dir)
			continue
		}
		if err := module.CheckCacheTree(dir); err != nil {
			bad++
			fmt.Fprintf(io.Out, "%s %s: %s\n", name, e.Version, err)
			continue
		}
		hash, err := module.TreeHash(dir)
		if err != nil {
			return errors.WrapErr(err, "hash dependency")
		}
		if hash == e.Hash {
			fmt.Fprintf(io.Out, "%s %s: ok\n", name, e.Version)
		} else {
			bad++
			fmt.Fprintf(io.Out, "%s %s: MISMATCH in %s\n", name, e.Version, dir)
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
