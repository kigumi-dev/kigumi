package mir

import "testing"

func TestPlatformProvides(t *testing.T) {
	hosted := func(os string) Platform { return Platform{OS: os} }
	cases := []struct {
		name string
		p    Platform
		want map[Capability]bool
	}{
		{"linux", hosted("linux"), map[Capability]bool{
			CapHostShell: true, CapHostArgs: true, CapHostFiles: true, CapHostNet: true, CapHostDL: true,
			CapHostEntropy: true, CapHostSignals: true, CapHostStdio: true, CapTaskBlocking: true,
			CapTaskExecutor: true, CapTaskThreads: true, CapFFIRaw: true, CapSecureErase: true,
		}},
		{"windows has no POSIX signals", hosted("windows"), map[Capability]bool{CapHostSignals: false, CapHostShell: true, CapTaskThreads: true}},
		{"wasi is a subset", hosted("wasi"), map[Capability]bool{
			CapHostStdio: true, CapHostArgs: true, CapHostFiles: true, CapHostEntropy: true,
			CapHostShell: false, CapHostNet: false, CapHostDL: false, CapHostSignals: false,
			CapTaskExecutor: true, CapTaskThreads: false,
		}},
		{"freestanding", Platform{OS: "freestanding", Bare: true, NoLibc: true}, map[Capability]bool{
			CapHostStdio: false, CapHostArgs: false, CapHostShell: false, CapHostSignals: false,
			CapTaskBlocking: false, CapTaskThreads: false, CapTaskExecutor: true, CapFFICString: true, CapFFIHandleTable: true,
		}},
		{"hosted --sys none", Platform{OS: "linux", Bare: true}, map[Capability]bool{
			CapHostArgs: false, CapHostNet: false, CapTaskBlocking: true, CapTaskThreads: false, CapTaskExecutor: true,
		}},
		{"strict blocking profile", Platform{OS: "linux", ThreadProfile: "isr"}, map[Capability]bool{
			CapTaskThreads: false, CapTaskExecutor: true, CapHostArgs: true,
		}},
		{"allocator none changes nothing", Platform{OS: "linux", Allocator: "none"}, map[Capability]bool{
			CapHostArgs: true, CapTaskExecutor: true, CapFFIPin: true,
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
	capabilityNeeds[CapHostArgs] = []Capability{CapTaskThreads}
	defer delete(capabilityNeeds, CapHostArgs)
	got := Platform{OS: "wasi"}.Provides(CapHostArgs)
	if got.Provided || got.Reason != "needs task.threads: wasi has no worker threads" {
		t.Errorf("Provides(host.args needing task.threads) on wasi = %+v", got)
	}
}
