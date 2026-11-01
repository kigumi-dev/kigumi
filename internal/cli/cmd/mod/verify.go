package mod

import (
	"fmt"
	"os"
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
	for _, name := range sortedNames(lock) {
		e := lock.Entries[name]
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

func sortedNames(lock module.Lock) []string {
	names := make([]string, 0, len(lock.Entries))
	for n := range lock.Entries {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
