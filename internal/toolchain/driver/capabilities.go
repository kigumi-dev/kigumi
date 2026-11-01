package driver

import (
	opdef "kigumi/internal/compiler/mir/ops"
	mirreach "kigumi/internal/compiler/mir/reach"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/artifact"
	buildconfig "kigumi/internal/toolchain/config"
)

type CapabilityReason = artifact.CapabilityReason

// Capabilities is a.Prog's reachable Capability vocabulary; nil if a has
// no program (unanalyzed, or diagnostics stopped it before MIR).
func Capabilities(a *Analysis) []string {
	if a == nil || a.Prog == nil {
		return nil
	}
	return opdef.Capabilities(mirreach.Reachable(a.Prog.R, a.Prog.Entry, a.Prog.Funcs, a.Prog.ByEnt).Ops)
}

func CapabilityReasons(a *Analysis, t buildconfig.Target) []CapabilityReason {
	if a == nil || a.Prog == nil {
		return nil
	}
	return capabilityReasons(mirreach.Reachable(a.Prog.R, a.Prog.Entry, a.Prog.Funcs, a.Prog.ByEnt), a.Res, t)
}

// capabilityReasons is reach's capability report as the artifact's sidecar records.
func capabilityReasons(rs *mirreach.Reachability, res *sem.Result, t buildconfig.Target) []CapabilityReason {
	var out []CapabilityReason
	for _, r := range rs.CapabilityReasons(res, capabilityPlatform(t)) {
		out = append(out, capabilityRecord(r))
	}
	return out
}

func capabilityRecord(r mirreach.CapabilityReason) CapabilityReason {
	var sites []artifact.CapabilitySite
	for _, s := range r.Sites {
		sites = append(sites, artifact.CapabilitySite(s))
	}
	return CapabilityReason{Capability: r.Capability, Op: r.Op, RequiredBy: r.RequiredBy, RootKind: r.RootKind, Path: r.Path, Sites: sites, Provided: r.Provided, Why: r.Why}
}
