package driver

import (
	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
	buildconfig "kigumi/internal/toolchain/config"
)

type CapabilityReason = mir.CapabilityReason

// Capabilities is a.Prog's reachable Capability vocabulary; nil if a has
// no program (unanalyzed, or diagnostics stopped it before MIR).
func Capabilities(a *Analysis) []string {
	if a == nil || a.Prog == nil {
		return nil
	}
	return opdef.Capabilities(mir.Reachable(a.Prog).Ops)
}

func CapabilityReasons(a *Analysis, t buildconfig.Target) []CapabilityReason {
	if a == nil || a.Prog == nil {
		return nil
	}
	return mir.Reachable(a.Prog).CapabilityReasons(a.Res, capabilityPlatform(t))
}
