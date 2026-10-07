package driver

import "kigumi/internal/mir"

// The zero Target resolves as the host.
func (t Target) CapabilityPlatform() mir.Platform {
	if t.OS == "" {
		t = HostTarget()
	}
	p := mir.Platform{OS: t.OS, Bare: t.Bare(), NoLibc: t.Freestanding(), Allocator: t.Allocator}
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

func ProvidedCapabilities(t Target) []CapabilityVerdict {
	plat := t.CapabilityPlatform()
	var out []CapabilityVerdict
	for _, c := range mir.Vocabulary() {
		got := plat.Provides(c)
		out = append(out, CapabilityVerdict{Capability: string(c), Provided: got.Provided, Why: got.Reason})
	}
	return out
}
