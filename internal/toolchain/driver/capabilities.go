package driver

import (
	opdef "kigumi/internal/compiler/mir/ops"
	mirreach "kigumi/internal/compiler/mir/reach"
	buildconfig "kigumi/internal/toolchain/config"
)

type CapabilityReason = mirreach.CapabilityReason

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
	return mirreach.Reachable(a.Prog.R, a.Prog.Entry, a.Prog.Funcs, a.Prog.ByEnt).CapabilityReasons(a.Res, capabilityPlatform(t))
}
