package reach

import (
	"testing"

	opdef "kigumi/internal/compiler/mir/ops"
)

func TestPlatformProvides(t *testing.T) {
	hosted := func(os string) Platform { return Platform{OS: os} }
	cases := []struct {
		name string
		p    Platform
		want map[opdef.Capability]bool
	}{
		{"linux", hosted("linux"), map[opdef.Capability]bool{
			opdef.CapHostShell: true, opdef.CapHostArgs: true, opdef.CapHostFiles: true, opdef.CapHostNet: true, opdef.CapHostDL: true,
			opdef.CapHostEntropy: true, opdef.CapHostSignals: true, opdef.CapHostStdio: true, opdef.CapTaskBlocking: true,
			opdef.CapTaskExecutor: true, opdef.CapTaskThreads: true, opdef.CapFFIRaw: true, opdef.CapSecureErase: true,
		}},
		{"windows has no POSIX signals", hosted("windows"), map[opdef.Capability]bool{opdef.CapHostSignals: false, opdef.CapHostShell: true, opdef.CapTaskThreads: true}},
		{"wasi is a subset", hosted("wasi"), map[opdef.Capability]bool{
			opdef.CapHostStdio: true, opdef.CapHostArgs: true, opdef.CapHostFiles: true, opdef.CapHostEntropy: true,
			opdef.CapHostShell: false, opdef.CapHostNet: false, opdef.CapHostDL: false, opdef.CapHostSignals: false,
			opdef.CapTaskExecutor: true, opdef.CapTaskThreads: false,
		}},
		{"freestanding", Platform{OS: "freestanding", Bare: true, NoLibc: true}, map[opdef.Capability]bool{
			opdef.CapHostStdio: false, opdef.CapHostArgs: false, opdef.CapHostShell: false, opdef.CapHostSignals: false,
			opdef.CapTaskBlocking: false, opdef.CapTaskThreads: false, opdef.CapTaskExecutor: true, opdef.CapFFICString: true, opdef.CapFFIHandleTable: true,
		}},
		{"hosted --sys none", Platform{OS: "linux", Bare: true}, map[opdef.Capability]bool{
			opdef.CapHostArgs: false, opdef.CapHostNet: false, opdef.CapTaskBlocking: true, opdef.CapTaskThreads: false, opdef.CapTaskExecutor: true,
		}},
		{"strict blocking profile", Platform{OS: "linux", ThreadProfile: "isr"}, map[opdef.Capability]bool{
			opdef.CapTaskThreads: false, opdef.CapTaskExecutor: true, opdef.CapHostArgs: true,
		}},
		{"allocator none withholds the heap-backed capabilities", Platform{OS: "linux", Allocator: "none"}, map[opdef.Capability]bool{
			opdef.CapHostArgs: true, opdef.CapFFIPin: true, opdef.CapSecureErase: true, opdef.CapFFIRaw: true,
			opdef.CapTaskExecutor: false, opdef.CapTaskThreads: false, opdef.CapTaskBlocking: false, opdef.CapFFICString: false, opdef.CapFFIHandleTable: false,
		}},
		{"recoverable-only withholds the same set", Platform{OS: "linux", RecoverableOnly: true}, map[opdef.Capability]bool{
			opdef.CapHostArgs: true, opdef.CapFFIPin: true, opdef.CapSecureErase: true,
			opdef.CapTaskExecutor: false, opdef.CapTaskThreads: false, opdef.CapTaskBlocking: false, opdef.CapFFICString: false, opdef.CapFFIHandleTable: false,
		}},
	}
	for _, c := range cases {
		for cap, want := range c.want {
			got := c.p.Provides(cap)
			if got.Provided != want {
				t.Errorf("%s: Provides(%s) = %+v, want provided=%v", c.name, cap, got, want)
			}
			if !got.Provided && got.Reason == "" {
				t.Errorf("%s: withheld %s with no reason", c.name, cap)
			}
		}
	}
}

func TestProvidesFollowsNeeds(t *testing.T) {
	p := Platform{OS: "linux", Allocator: "none"}
	got := p.Provides(opdef.CapTaskThreads)
	want := "needs task.executor: its runtime state is heap-allocated outside any allocator block, and this build has no default allocator (--allocator none)"
	if got.Provided || got.Reason != want {
		t.Errorf("Provides(task.threads) without an executor = %+v", got)
	}
}
