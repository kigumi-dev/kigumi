package module

import (
	"fmt"
	"sort"

	buildconfig "kigumi/internal/toolchain/config"
)

// LockChange is a locked module whose selected version differs from the lock's.
type LockChange struct{ Name, Old, New string }

func (c LockChange) String() string { return fmt.Sprintf("%s %s -> %s", c.Name, c.Old, c.New) }

// PruneLock keeps the entries of lk that agree with minimal version selection over mf. Entries are keyed by effective Require key, so aliases are pruned independently. Entries for modules that are no longer selected are dropped; entries whose selected version changed are dropped and listed, since only `get` may lock a new version.
func PruneLock(o buildconfig.CompilerOptions, mf ModFile, local Local, lk Lock) (Lock, []LockChange, error) {
	sel, err := SelectVersions(o, mf, local)
	if err != nil {
		return lk, nil, err
	}
	next := Lock{Entries: map[string]LockEntry{}}
	var changes []LockChange
	for key, e := range lk.Entries {
		r, ok := sel.Requires[key]
		if _, replaced := local.Replacement(e.Require()); replaced {
			next.Entries[key] = e
			continue
		}
		switch {
		case !ok:
		case e.Version != r.Version:
			changes = append(changes, LockChange{key, e.Version, r.Version})
		default:
			next.Entries[key] = e
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })
	return next, changes, nil
}

// RetainLockState keeps the bindings of state whose effective key is still in
// kept, plus the Source and Resolution records they reference. Resolutions of
// a retained source stay even when their request lost selection: they are the
// offline re-selection evidence a later migration must not drop. inputs
// replaces the header digest because the caller changed the manifest bytes.
func RetainLockState(state LockState, kept Lock, inputs string) LockState {
	out := state.Clone()
	out.Inputs = inputs
	out.Locks = nil
	out.Sources = nil
	out.Resolutions = nil
	out.Edges = nil
	sourceIDs := map[string]bool{}
	for _, b := range state.Locks {
		if _, ok := kept.Entries[b.Key()]; !ok {
			continue
		}
		out.Locks = append(out.Locks, b)
		if b.SourceID != "" {
			sourceIDs[b.SourceID] = true
		}
	}
	for _, src := range state.Sources {
		if sourceIDs[src.ID] {
			out.Sources = append(out.Sources, src)
		}
	}
	for _, res := range state.Resolutions {
		if sourceIDs[res.SourceID] {
			out.Resolutions = append(out.Resolutions, res)
		}
	}
	for _, e := range state.Edges {
		_, from := kept.Entries[e.From]
		_, to := kept.Entries[e.To]
		if from && to {
			out.Edges = append(out.Edges, e)
		}
	}
	out.SortRecords()
	return out
}

// LockDisagreements lists, sorted, where lk differs from what selection over mf picks, as `run` would see it. Entries are matched by effective Require key, so an alias is checked against its own locked version.
func LockDisagreements(o buildconfig.CompilerOptions, mf ModFile, local Local, lk Lock) ([]string, error) {
	sel, err := SelectVersions(o, mf, local)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, key := range sel.Names() {
		r := sel.Requires[key]
		if _, replaced := local.Replacement(r); replaced {
			continue
		}
		e, ok := lk.Entries[key]
		switch {
		case !ok:
			out = append(out, fmt.Sprintf("%s %s: selected but not in the lock; run `kigumi get`", key, r.Version))
		case e.Name != "" && e.Name != r.Name:
			out = append(out, fmt.Sprintf("%s: the lock names %s but the Require names %s; run `kigumi get`", key, e.Name, r.Name))
			continue
		case e.Alias != r.Alias:
			out = append(out, fmt.Sprintf("%s: the lock has alias %q but the Require has alias %q; run `kigumi get`", key, e.Alias, r.Alias))
			continue
		case e.Version != r.Version:
			out = append(out, fmt.Sprintf("%s: the lock has %s but selection picks %s; run `kigumi get`", key, e.Version, r.Version))
		default:
			if want := RepoURLKey(r); e.URL != "" && want != "" && e.URL != want {
				out = append(out, fmt.Sprintf("%s %s: the lock recorded %s but the Require now says %s; run `kigumi get` to accept the new source", key, r.Version, e.URL, want))
			}
		}
	}
	for key, e := range lk.Entries {
		if _, ok := sel.Requires[key]; !ok {
			label := key
			if e.Name != "" && e.Name != key {
				label = key + " (" + e.Name + ")"
			}
			out = append(out, fmt.Sprintf("%s %s: locked but no longer selected; run `kigumi get`", label, e.Version))
		}
	}
	sort.Strings(out)
	return out, nil
}
