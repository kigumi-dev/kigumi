package driver

import (
	"crypto/sha256"
	"fmt"

	"kigumi/internal/buildgraph"
	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
)

// PlanStats counts how many artifact analyses a plan ran and how many it reused.
type PlanStats struct {
	Analyses, Reused int
}

// planFacts is what a plan reads from one artifact's analysis; reach is nil when the checker rejected the module.
type planFacts struct {
	provenance []schema.GeneratedArtifact
	res        *sem.Result
	reach      *mir.Reachability
}

type planCache struct {
	facts map[string]*planFacts
	stats PlanStats
}

func newPlanCache() *planCache { return &planCache{facts: map[string]*planFacts{}} }

// artifactFacts scopes to a's own entry set so a sibling artifact's usage never leaks in.
func (c *planCache) artifactFacts(p *BuildProgram, a *buildgraph.Artifact, t Target) *planFacts {
	m, err := LoadModule(p.Module.Root, LoadOptions{StdRoot: p.stdRoot, Target: t, Deny: a.Deny, Ceiling: a.Ceiling, ArtifactName: a.Name, VerifyPasses: p.Module.verifyPasses})
	if err != nil {
		return nil
	}
	entry, err := scriptUnder(m, a.Dir)
	if err != nil {
		return nil
	}
	m.Entry = entry
	m.ExplicitEntry = true
	mod, err := m.semModule()
	if err != nil {
		return nil
	}
	key := analysisKey(m, mod)
	if f, ok := c.facts[key]; ok {
		c.stats.Reused++
		return f
	}
	c.stats.Analyses++
	an, err := m.analyze(mod, nil)
	if err != nil {
		return nil
	}
	f := &planFacts{provenance: an.Provenance, res: an.Res}
	if an.Prog != nil {
		f.reach = mir.Reachable(an.Prog)
	}
	c.facts[key] = f
	return f
}

func (f *planFacts) reasons(t Target) []mir.CapabilityReason {
	if f == nil || f.reach == nil {
		return nil
	}
	return f.reach.CapabilityReasons(f.res, t.CapabilityPlatform())
}

// analysisKey omits the artifact's name, which only labels diagnostics.
func analysisKey(m *Module, mod *sem.Module) string {
	h := sha256.New()
	w := func(format string, args ...any) { fmt.Fprintf(h, format+"\n", args...) }
	w("%d %v %v %v %q %d %q %q %q", mod.Target, mod.NoDefaultAllocator, mod.RecoverableOnly, mod.NoPlatform, mod.BlockingProfile, mod.PtrBits, mod.Arch, mod.Deny, mod.LayerCeiling)
	w("%q %v %d", m.Entry, m.target.StrictStack, m.target.StackBudgetKiB)
	for _, pkg := range mod.Packages {
		w("pkg %q %q %v", pkg.Path, pkg.Module, pkg.Std)
		for _, t := range pkg.Files {
			w("file %q %v %d", t.File.Name, t.File.Generated, len(t.File.Src))
			h.Write(t.File.Src)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
