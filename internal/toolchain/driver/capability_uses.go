package driver

import (
	"fmt"

	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
	mirreach "kigumi/internal/compiler/mir/reach"
	"kigumi/internal/compiler/sem"
	buildconfig "kigumi/internal/toolchain/config"
)

func capabilityUses(p *mir.Program, reach *mirreach.Reachability, plat mirreach.Platform) []sem.CapabilityUse {
	var out []sem.CapabilityUse
	reported := map[opdef.Capability]bool{}
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
			Op: opdef.StdOpNames[op], Capability: string(c), Reason: got.Reason,
			File: caller.File, Node: site.Node, Path: reach.PathNames(p.R, site.Caller),
		})
	}
	return out
}

func (a *Analysis) checkProvision(t buildconfig.Target) *mirreach.Reachability {
	key := fmt.Sprintf("%+v", t)
	if a.provision == nil {
		a.provision = map[string]*mirreach.Reachability{}
	}
	if reach, ok := a.provision[key]; ok {
		return reach
	}
	reach := mirreach.Reachable(a.Prog.R, a.Prog.Entry, a.Prog.Funcs, a.Prog.ByEnt)
	// A published analysis is read-only: a target first seen after publication
	// still gets its reachability, but cannot add diagnostics to the snapshot.
	if !a.Res.Sealed() {
		a.Res.CheckCapabilityProvision(capabilityUses(a.Prog, reach, capabilityPlatform(t)))
		a.warnTaskPanicFatal(t, reach)
	}
	a.provision[key] = reach
	return reach
}
