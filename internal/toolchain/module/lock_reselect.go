package module

import (
	"fmt"
	"sort"

	buildconfig "kigumi/internal/toolchain/config"
)

// LockChange is a locked module whose selected version differs from the lock's.
type LockChange struct{ Name, Old, New string }

func (c LockChange) String() string { return fmt.Sprintf("%s %s -> %s", c.Name, c.Old, c.New) }

// PruneLock keeps the entries of lk that agree with minimal version selection over mf. Entries for modules that are no longer selected are dropped; entries whose selected version changed are dropped and listed, since only `get` may lock a new version.
func PruneLock(o buildconfig.CompilerOptions, mf ModFile, local Local, lk Lock) (Lock, []LockChange, error) {
	sel, err := SelectVersions(o, mf, local)
	if err != nil {
		return lk, nil, err
	}
	next := Lock{Entries: map[string]LockEntry{}}
	var changes []LockChange
	for name, e := range lk.Entries {
		r, ok := sel.Requires[name]
		if _, replaced := local.Replaces[name]; replaced {
			next.Entries[name] = e
			continue
		}
		switch {
		case !ok:
		case e.Version != r.Version:
			changes = append(changes, LockChange{name, e.Version, r.Version})
		default:
			next.Entries[name] = e
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })
	return next, changes, nil
}

// LockDisagreements lists, sorted, where lk differs from what selection over mf picks, as `run` would see it.
func LockDisagreements(o buildconfig.CompilerOptions, mf ModFile, local Local, lk Lock) ([]string, error) {
	sel, err := SelectVersions(o, mf, local)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, name := range sel.Names() {
		r := sel.Requires[name]
		if _, replaced := local.Replaces[name]; replaced {
			continue
		}
		e, ok := lk.Entries[name]
		switch {
		case !ok:
			out = append(out, fmt.Sprintf("%s %s: selected but not in the lock; run `kigumi get`", name, r.Version))
		case e.Version != r.Version:
			out = append(out, fmt.Sprintf("%s: the lock has %s but selection picks %s; run `kigumi get`", name, e.Version, r.Version))
		default:
			if want := RepoURLKey(r); e.URL != "" && want != "" && e.URL != want {
				out = append(out, fmt.Sprintf("%s %s: the lock recorded %s but the Require now says %s; run `kigumi get` to accept the new source", name, r.Version, e.URL, want))
			}
		}
	}
	for name, e := range lk.Entries {
		if _, ok := sel.Requires[name]; !ok {
			out = append(out, fmt.Sprintf("%s %s: locked but no longer selected; run `kigumi get`", name, e.Version))
		}
	}
	sort.Strings(out)
	return out, nil
}
