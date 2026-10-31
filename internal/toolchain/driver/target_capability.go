package driver

import (
	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
	buildconfig "kigumi/internal/toolchain/config"
)

func capabilityPlatform(t buildconfig.Target) mir.Platform {
	p := mir.Platform{OS: t.OS, Bare: t.Bare(), NoLibc: t.Freestanding(), Allocator: t.Allocator, RecoverableOnly: t.RecoverableOnly()}
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
