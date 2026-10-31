package driver

import (
	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
	buildconfig "kigumi/internal/toolchain/config"
)

func capabilityUses(p *mir.Program, reach *mir.Reachability, plat mir.Platform) []sem.CapabilityUse {
	var out []sem.CapabilityUse
	reported := map[mir.Capability]bool{}
	for _, op := range reach.Ops {
		c, ok := op.Capability()
		if !ok {
			continue
		}
		got := plat.Provides(c)
		site := reach.OpSites[op]
		caller := p.ByEnt[site.Caller]
		if got.Provided || caller == nil || reported[c] {
			continue
		}
		reported[c] = true
		out = append(out, sem.CapabilityUse{
			Op: mir.StdOpNames[op], Capability: string(c), Reason: got.Reason,
			File: caller.File, Node: site.Node, Path: reach.PathNames(p.R, site.Caller),
		})
	}
	return out
}

func (a *Analysis) checkProvision(t buildconfig.Target) *mir.Reachability {
	reach := mir.Reachable(a.Prog)
	a.Res.CheckCapabilityProvision(capabilityUses(a.Prog, reach, capabilityPlatform(t)))
	a.warnTaskPanicFatal(t, reach)
	return reach
}
