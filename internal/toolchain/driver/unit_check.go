package driver

import (
	"fmt"
	"sort"
	"strings"

	"kigumi/internal/compiler/sem"
)

// CheckUnits is Check for a caller that only needs the diagnostics.
func CheckUnits(m *Module) (*sem.Result, error) {
	if m.settled != nil || !m.unitEligible() {
		return Check(m)
	}
	if res, ok := m.checkUnits(); ok {
		return res, nil
	}
	m.unitStats.Fallbacks++
	return Check(m)
}

type unitPlan struct {
	m        *Module
	g        *pkgGraph
	env, std string
	digests  map[string]string
	done     map[int]bool
	status   map[string]PackageStatus
	texts    map[string]string
	warmed   bool
	runs     int
	upward   bool
	anyErr   bool
	comptime bool
	// replayedC holds the C link names the replayed records carry, so a run that declares one of them can ask for the whole module.
	replayedC map[string]bool
}

func (m *Module) checkUnits() (*sem.Result, bool) {
	defer m.saveMemo()
	g, err := m.keyGraph()
	if err != nil || !m.unitGraphUsable(g) {
		return nil, false
	}
	std, err := m.stdDigest()
	if err != nil {
		return nil, false
	}
	u := &unitPlan{m: m, g: g, env: m.cacheEnv(), std: std, digests: map[string]string{}, done: map[int]bool{}, status: map[string]PackageStatus{}, texts: map[string]string{}}
	m.pkgStatus = nil
	u.warmed = u.warm()
	for {
		missed, pending := u.resolve()
		if len(missed) == 0 {
			break
		}
		batch := missed
		if u.runs > 0 || !u.warmed {
			batch = append(batch, pending...)
		}
		// A partial program can trip a compiler fault that the whole module's diagnostics would hide, so a fault falls back too.
		if err := u.run(batch); err != nil || u.upward || u.anyErr && len(pending) > 0 {
			return nil, false
		}
	}
	if u.runs > 1 && u.anyErr {
		return nil, false
	}
	if !u.warmed {
		u.markWarm()
	}
	return u.finish(), true
}

type warmMarker struct{}

// warm says an earlier CheckUnits of this module under the same environment and std finished; a cold start checks everything in one run instead of one run per layer.
func (u *unitPlan) warm() bool {
	return u.m.pkgCache.getSealed("unit", u.markerName(), &warmMarker{})
}

func (u *unitPlan) markWarm() { u.m.pkgCache.putSealed("unit", u.markerName(), warmMarker{}) }

func (u *unitPlan) markerName() string {
	m := u.m
	return sumHex([]byte(fmt.Sprintf("%s|%s|%v|%v|%s|%s", m.Root, m.keyEntry, m.keyExplicit, m.test, u.env, u.std)))
}

// resolve replays every component whose dependencies are settled and whose record is clean; the rest come back split by whether their keys can be computed yet.
func (u *unitPlan) resolve() (missed, pending []int) {
	for i, comp := range u.g.sccs {
		switch {
		case u.done[i]:
		case !u.depsSettled(comp):
			pending = append(pending, i)
		case u.replay(i):
			u.done[i] = true
		default:
			missed = append(missed, i)
		}
	}
	return missed, pending
}

func (u *unitPlan) depsSettled(comp []string) bool {
	for _, p := range comp {
		for _, dep := range u.g.deps[p] {
			if j := u.g.sccOf[dep]; j != u.g.sccOf[p] && !u.done[j] {
				return false
			}
		}
	}
	return true
}

func (u *unitPlan) replay(i int) bool {
	c := u.m.pkgCache
	recs := map[string]keyRecord{}
	keys := map[string]string{}
	for _, p := range u.g.sccs[i] {
		key, err := u.m.packageKey(u.g, p, u.env, u.std, u.digests)
		if err != nil || !u.g.depsUsable(p, u.digests) {
			return false
		}
		rec, ok := c.lookup(key)
		if !ok || !rec.Clean {
			return false
		}
		recs[p], keys[p] = rec, key
	}
	for p, rec := range recs {
		c.stats.ArtifactHits++
		u.digests[p] = rec.Digest
		u.status[p] = PackageStatus{Path: p, Key: keys[p], Digest: rec.Digest, Hit: true, Replayed: true, Clean: true}
		u.m.unitStats.bump(&u.m.unitStats.Replayed, p)
		for _, name := range rec.CSymbols {
			if u.replayedC == nil {
				u.replayedC = map[string]bool{}
			}
			u.replayedC[name] = true
		}
	}
	return true
}

// finish composes the diagnostics in package order, the order a whole check prints them in.
func (u *unitPlan) finish() *sem.Result {
	m := u.m
	for _, comp := range u.g.sccs {
		for _, p := range comp {
			m.pkgStatus = append(m.pkgStatus, u.status[p])
		}
	}
	paths := make([]string, 0, len(u.texts))
	for p := range u.texts {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var sb strings.Builder
	for _, p := range paths {
		sb.WriteString(u.texts[p])
	}
	render := sb.String()
	if !u.anyErr && !u.comptime {
		m.putSettled(render)
	}
	return sem.ReplayResult(render, u.anyErr)
}
