package driver

import (
	"strings"

	"kigumi/internal/compiler/mir"
)

// unitEligible limits replay to targets where no whole-program rule reports at a library site on behalf of an entry: no profile, no stack budget, every capability provided.
func (m *Module) unitEligible() bool {
	t := m.target
	if m.pkgCache == nil || !m.unitCheck || m.lowerAll || m.verifyPasses {
		return false
	}
	if t.Bare() || t.NoDefaultAllocator() || t.RecoverableOnly() || t.StrictBlocking() || t.BlockingExecutor != "" || t.StrictStack || t.StackBudgetKiB != 0 || !wantsTaskCatch(t) {
		return false
	}
	plat := t.CapabilityPlatform()
	for _, c := range mir.Vocabulary() {
		if !plat.Provides(c).Provided {
			return false
		}
	}
	return len(m.Links) == 0 && len(m.Sources) == 0 && m.entryIsKeyed() && !m.HasParseErrors()
}

// entryIsKeyed says which file is the entry depends on nothing a package key misses.
func (m *Module) entryIsKeyed() bool {
	if m.keyExplicit {
		return true
	}
	for _, path := range m.index.order {
		if ip := m.index.pkgs[path]; !ip.std && ip.module == "" && strings.HasPrefix(m.Rel(path), "cmd/") {
			return true
		}
	}
	return false
}

// unitGraphUsable rejects a dependency module (its bodies are lowered only when a dependant reaches them), an unresolved import, and a package the graph does not cover.
func (m *Module) unitGraphUsable(g *pkgGraph) bool {
	if len(g.unresolved) != 0 {
		return false
	}
	inGraph := map[string]bool{}
	for _, p := range g.nodes {
		inGraph[p] = true
		if m.index.pkgs[p].module != "" {
			return false
		}
	}
	for path, p := range m.Packages {
		if p.Header || (!p.Std && !inGraph[path]) {
			return false
		}
	}
	return true
}
