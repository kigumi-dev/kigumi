package driver

import "kigumi/internal/sem"

// analyzeForCheck is Analyze for a caller that needs only diagnostics, so units allow lazy std bodies too.
func (m *Module) analyzeForCheck() (*Analysis, error) {
	if m.unitCheck && !m.lazyStd && !m.lowerAll {
		if a, ok, err := m.analyzeLazyStd(nil); ok || err != nil {
			return a, err
		}
	}
	return Analyze(m)
}

// analyzeLazyStd analyzes with the std bodies the module never names left unchecked; ok is false when a skipped body was needed after all.
func (m *Module) analyzeLazyStd(gen sem.DeriveGenerator) (*Analysis, bool, error) {
	mod, err := m.semModule()
	if err != nil {
		return nil, false, err
	}
	mod.LazyBodies = func(path string) bool { p := m.Packages[path]; return p != nil && p.Std }
	a, err := m.analyzeCore(mod, gen, nil)
	if err != nil || !lazyComplete(a) {
		m.unitStats.LazyRetries++
		return nil, false, nil
	}
	m.unitStats.Lazy++
	m.unitStats.SkippedBodies += a.Res.SkippedBodies()
	m.recordPackages(a.Res, a.Prog != nil)
	return a, true, nil
}

// lazyComplete says a run that skipped bodies never needed one of them.
func lazyComplete(a *Analysis) bool {
	return !a.Res.LazyMissed() && (a.Prog == nil || len(a.Prog.Skipped) == 0)
}

// EveryBody makes later checks of m check every std body, for a caller that reads facts of a function the module never names.
func (m *Module) EveryBody() { m.lazyStd, m.unitCheck = false, false }
