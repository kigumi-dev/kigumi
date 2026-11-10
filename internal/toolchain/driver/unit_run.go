package driver

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// run checks the components together with the imports they need in one semantic run, then records each package.
func (u *unitPlan) run(batch []int) error {
	m := u.m
	unit := map[string]bool{}
	include := map[string]bool{}
	for _, i := range batch {
		for _, p := range u.g.sccs[i] {
			unit[p], include[p] = true, true
			for _, dep := range u.g.closure(p) {
				include[dep] = true
			}
		}
	}
	a, lazy, err := u.analyze(unit, include)
	if err != nil {
		return err
	}
	u.runs++
	m.unitStats.Runs++
	res := a.Res
	if u.hasReplayedC(unitCSymbols(res, unit)) {
		// E2218 is a rule about the whole module (one definition and one signature per C name), so a unit that declares a name a replayed
		// package also declares cannot be judged without it: the check goes to the whole module, which sees both sides.
		u.upward = true
		return nil
	}
	if res.HasErrors() {
		u.anyErr = true
	}
	if len(res.Comptime) > 0 {
		u.comptime = true
	}
	for id := 1; id < len(res.Packages); id++ {
		// A lazy run reports nothing outside its units, so an earlier run's text for an import stands.
		if path := res.Packages[id].Path; unit[path] || !lazy {
			u.texts[path] = res.RenderPackage(path)
		}
	}
	for p := range include {
		if unit[p] {
			m.unitStats.bump(&m.unitStats.Checked, p)
			continue
		}
		m.unitStats.bump(&m.unitStats.Context, p)
		if total, _ := res.PackageDiagCounts(p); total > 0 && u.status[p].Clean {
			u.demote(p)
		}
	}
	u.settle(batch, unitOutcome{res: res, mir: a.Prog != nil, comptime: res.ComptimePackages()})
	return nil
}

func (u *unitPlan) settle(batch []int, o unitOutcome) {
	for _, i := range sortedInts(batch) {
		for _, p := range u.g.sccs[i] {
			key, err := u.m.packageKey(u.g, p, u.env, u.std, u.digests)
			if err != nil {
				u.digests[p] = "absent"
				continue
			}
			st := u.m.recordPackage(o, p, key, u.g.depsUsable(p, u.digests))
			u.digests[p] = st.Digest
			u.status[p] = st
		}
		u.done[i] = true
	}
}

func sortedInts(in []int) []int {
	out := slices.Clone(in)
	slices.Sort(out)
	return out
}

// unitCSymbols lists the C link names this run's own packages declare: the names E2218 keys its per-module map on.
func unitCSymbols(res *sem.Result, unit map[string]bool) []string {
	var names []string
	for id := range res.Entities {
		e := &res.Entities[id]
		if e.Kind != sem.EntFn || e.File == 0 || int(e.Pkg) >= len(res.Packages) || !unit[res.Packages[e.Pkg].Path] {
			continue
		}
		info := res.Fn(sem.EntityID(id))
		if info.Abi != "" || info.Export != "" {
			names = append(names, res.FnLinkNameOf(sem.EntityID(id)))
		}
	}
	return names
}

// hasReplayedC says one of these names is also declared by a package this plan answered from its record.
func (u *unitPlan) hasReplayedC(names []string) bool {
	for _, name := range names {
		if u.replayedC[name] {
			return true
		}
	}
	return false
}

// demote withdraws the clean record of a package that reported something after all, and sends the check to the whole module.
func (u *unitPlan) demote(p string) {
	u.upward = true
	st := u.status[p]
	st.Clean, st.Replayed = false, false
	u.status[p] = st
	u.m.pkgCache.putSealed("pkg", st.Key, keyRecord{Path: p, Digest: st.Digest})
	u.m.unitStats.Demoted++
}

// analyze checks the unit's bodies and only the import bodies they name, and checks every body again when that was not enough.
func (u *unitPlan) analyze(unit, include map[string]bool) (*Analysis, bool, error) {
	if a, err := u.analyzeWith(unit, include, true); err == nil && lazyComplete(a) {
		u.m.unitStats.Lazy++
		u.m.unitStats.SkippedBodies += a.Res.SkippedBodies()
		return a, true, nil
	}
	u.m.unitStats.LazyRetries++
	a, err := u.analyzeWith(unit, include, false)
	return a, false, err
}

func (u *unitPlan) analyzeWith(unit, include map[string]bool, lazy bool) (*Analysis, error) {
	mod, err := u.m.semModuleOf(func(path string) bool { return include[path] })
	if err != nil {
		return nil, err
	}
	if lazy {
		mod.LazyBodies = func(path string) bool { return !unit[path] }
	}
	return u.m.analyzeCore(mod, nil, func(path string) bool { return unit[path] })
}
