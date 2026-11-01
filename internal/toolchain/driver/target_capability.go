package driver

import (
	opdef "kigumi/internal/compiler/mir/ops"
	mirreach "kigumi/internal/compiler/mir/reach"
	"kigumi/internal/toolchain/artifact"
	buildconfig "kigumi/internal/toolchain/config"
)

func capabilityPlatform(t buildconfig.Target) mirreach.Platform {
	p := mirreach.Platform{OS: t.OS, Bare: t.Bare(), NoLibc: t.Freestanding(), Allocator: t.Allocator, RecoverableOnly: t.RecoverableOnly()}
	if t.StrictBlocking() {
		p.ThreadProfile = t.BlockingExecutor
	}
	return p
}

func ProvidedCapabilities(t buildconfig.Target) []artifact.CapabilityVerdict {
	plat := capabilityPlatform(t)
	var out []artifact.CapabilityVerdict
	for _, c := range opdef.Vocabulary() {
		got := plat.Provides(c)
		out = append(out, artifact.CapabilityVerdict{Capability: string(c), Provided: got.Provided, Why: got.Reason})
	}
	return out
}
