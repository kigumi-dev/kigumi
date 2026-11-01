package driver

import (
	opdef "kigumi/internal/compiler/mir/ops"
	mirreach "kigumi/internal/compiler/mir/reach"
	buildconfig "kigumi/internal/toolchain/config"
)

func capabilityPlatform(t buildconfig.Target) mirreach.Platform {
	p := mirreach.Platform{OS: t.OS, Bare: t.Bare(), NoLibc: t.Freestanding(), Allocator: t.Allocator, RecoverableOnly: t.RecoverableOnly()}
	if t.StrictBlocking() {
		p.ThreadProfile = t.BlockingExecutor
	}
	return p
}

type CapabilityVerdict struct {
	Capability string `json:"capability"`
	Provided   bool   `json:"provided"`
	Why        string `json:"why,omitempty"`
}

func ProvidedCapabilities(t buildconfig.Target) []CapabilityVerdict {
	plat := capabilityPlatform(t)
	var out []CapabilityVerdict
	for _, c := range opdef.Vocabulary() {
		got := plat.Provides(c)
		out = append(out, CapabilityVerdict{Capability: string(c), Provided: got.Provided, Why: got.Reason})
	}
	return out
}
