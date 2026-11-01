package mod

import (
	"fmt"
	"os"
	"sort"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
)

func runVerify(io session.IO, opts options) error {
	lock, err := driver.ReadLock(opts.root)
	if err != nil {
		return errors.WrapErr(err, "read lock")
	}
	bad := 0
	for _, name := range sortedNames(lock) {
		e := lock.Entries[name]
		dir, err := driver.CacheDir(session.CompilerOptions(), driver.Require{Name: name, Version: e.Version})
		if err != nil {
			return err
		}
		if _, err := os.Stat(dir); err != nil {
			bad++
			fmt.Fprintf(io.Msg, "%s %s: not fetched\n", name, e.Version)
			continue
		}
		if !driver.CacheReady(dir) {
			bad++
			fmt.Fprintf(io.Msg, "%s %s: incomplete cache entry in %s (missing origin marker or manifest)\n", name, e.Version, dir)
			continue
		}
		if err := driver.CheckCacheTree(dir); err != nil {
			bad++
			fmt.Fprintf(io.Msg, "%s %s: %s\n", name, e.Version, err)
			continue
		}
		hash, err := driver.TreeHash(dir)
		if err != nil {
			return errors.WrapErr(err, "hash dependency")
		}
		if hash == e.Hash {
			fmt.Fprintf(io.Msg, "%s %s: ok\n", name, e.Version)
		} else {
			bad++
			fmt.Fprintf(io.Msg, "%s %s: MISMATCH in %s\n", name, e.Version, dir)
		}
	}
	if bad > 0 {
		return exit.Code(1)
	}
	return nil
}

func sortedNames(lock driver.Lock) []string {
	names := make([]string, 0, len(lock.Entries))
	for n := range lock.Entries {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
