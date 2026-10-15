package driver

import "slices"

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
	mod, err := m.semModuleOf(func(path string) bool { return include[path] })
	if err != nil {
		return err
	}
	a, err := m.analyzeCore(mod, nil, func(path string) bool { return unit[path] })
	if err != nil {
		return err
	}
	u.runs++
	m.unitStats.Runs++
	res := a.Res
	if res.HasErrors() {
		u.anyErr = true
	}
	if len(res.Comptime) > 0 {
		u.comptime = true
	}
	for id := 1; id < len(res.Packages); id++ {
		u.texts[res.Packages[id].Path] = res.RenderPackage(res.Packages[id].Path)
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

// demote withdraws the clean record of a package that reported something after all, and sends the check to the whole module.
func (u *unitPlan) demote(p string) {
	u.upward = true
	st := u.status[p]
	st.Clean, st.Replayed = false, false
	u.status[p] = st
	u.m.pkgCache.putSealed("pkg", st.Key, keyRecord{Path: p, Digest: st.Digest})
	u.m.unitStats.Demoted++
}
