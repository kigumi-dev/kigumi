package driver

import (
	"crypto/sha256"
	"fmt"

	mirreach "kigumi/internal/compiler/mir/reach"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/artifact"
	"kigumi/internal/toolchain/buildgraph"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/contract/schema"
	"kigumi/internal/toolchain/module"
)

// PlanStats counts how many artifact analyses a plan ran and how many it reused.
type PlanStats struct {
	Analyses, Reused int
	// DeriveHits and DeriveMisses count derive generator runs served from or missed by the on-disk derive cache.
	DeriveHits, DeriveMisses int
	// Provisions and ProvisionReused count capability provision tables built or shared across targets of one OS class.
	Provisions, ProvisionReused int
	// EntryErrors lists artifacts whose directory does not name exactly one entry file; the build cannot run them.
	EntryErrors []string
}

// planFacts is what a plan reads from one artifact's analysis; reach is nil when the checker rejected the module.
type planFacts struct {
	provenance []schema.GeneratedArtifact
	res        *sem.Result
	reach      *mirreach.Reachability
}

type planCache struct {
	facts      map[string]*planFacts
	provisions map[string][]artifact.CapabilityVerdict
	derive     *schema.DeriveCache
	stats      PlanStats
}

func newPlanCache(o buildconfig.CompilerOptions) *planCache {
	c := &planCache{facts: map[string]*planFacts{}, provisions: map[string][]artifact.CapabilityVerdict{}}
	if dir, err := module.DeriveCacheDir(o); err == nil {
		c.derive = &schema.DeriveCache{Dir: dir}
	}
	return c
}

func (c *planCache) generator() sem.DeriveGenerator {
	if c.derive == nil {
		return nil
	}
	return c.derive.Generate
}

func (c *planCache) finish() PlanStats {
	if c.derive != nil {
		c.stats.DeriveHits, c.stats.DeriveMisses = c.derive.Hits(), c.derive.Misses()
	}
	return c.stats
}

// provision shares the verdict table between targets whose platforms differ only in an OS the rules treat alike.
func (c *planCache) provision(t buildconfig.Target) []artifact.CapabilityVerdict {
	key := capabilityPlatform(t).ProvisionKey()
	if v, ok := c.provisions[key]; ok {
		c.stats.ProvisionReused++
		return v
	}
	c.stats.Provisions++
	v := ProvidedCapabilities(t)
	c.provisions[key] = v
	return v
}

// artifactFacts scopes to a's own entry set so a sibling artifact's usage never leaks in.
func (c *planCache) artifactFacts(p *BuildProgram, a *buildgraph.Artifact, t buildconfig.Target) *planFacts {
	lo := p.Module.loadOptions()
	lo.Target = t
	lo.Deny, lo.Ceiling, lo.ArtifactName = a.Deny, a.Ceiling, a.Name
	lo.RootDirs = []string{a.Dir}
	m, err := LoadModule(p.Module.Root, lo)
	if err != nil {
		return nil
	}
	entry, err := scriptUnder(m, a.Dir)
	if err != nil {
		c.stats.EntryErrors = append(c.stats.EntryErrors, fmt.Sprintf("artifact %s: %s", a.Name, err))
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
	an, err := m.analyze(mod, c.generator())
	if err != nil {
		return nil
	}
	f := &planFacts{provenance: an.Provenance, res: an.Res}
	if an.Prog != nil {
		f.reach = mirreach.Reachable(an.Prog.R, an.Prog.Entry, an.Prog.Funcs, an.Prog.ByEnt)
	}
	c.facts[key] = f
	return f
}

func (f *planFacts) reasons(t buildconfig.Target) []CapabilityReason {
	if f == nil || f.reach == nil {
		return nil
	}
	return capabilityReasons(f.reach, f.res, t)
}

// analysisKey omits the artifact's name, which only labels diagnostics.
func analysisKey(m *Module, mod *sem.Module) string {
	h := sha256.New()
	w := func(format string, args ...any) { fmt.Fprintf(h, format+"\n", args...) }
	w("%d %v %v %v %q %d %q %q %q", mod.Target, mod.NoDefaultAllocator, mod.RecoverableOnly, mod.NoPlatform, mod.BlockingProfile, mod.PtrBits, mod.Arch, mod.Deny, mod.LayerCeiling)
	w("%q %v %d %q", m.Entry, m.target.StrictStack, m.target.StackBudgetKiB, mod.BuildProgram)
	w("isr %q %q %s", mod.OS, sem.InterruptKey(mod.Interrupt, mod.DepInterrupt), isrContractsVersion(mod))
	for _, pkg := range mod.Packages {
		w("pkg %q %q %v", pkg.Path, pkg.Module, pkg.Std)
		for _, t := range pkg.Files {
			w("file %q %v %d", t.File.Name, t.File.Generated, len(t.File.Src))
			h.Write(t.File.Src)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func isrContractsVersion(mod *sem.Module) string {
	if mod.IsrContracts == nil {
		return ""
	}
	return mod.IsrContracts.IsrVersion()
}
