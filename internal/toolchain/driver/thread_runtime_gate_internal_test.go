package driver

import (
	"testing"

	opdef "kigumi/internal/compiler/mir/ops"
	buildconfig "kigumi/internal/toolchain/config"
)

func TestTargetProvidesWorkerThreads(t *testing.T) {
	t.Parallel()
	hosted := buildconfig.Target{OS: "linux", Arch: "amd64", Sys: "posix"}
	cases := []struct {
		name   string
		target buildconfig.Target
		want   bool
	}{
		{"hosted", hosted, true},
		{"zero target is the host", buildconfig.Target{}, true},
		{"freestanding", buildconfig.Target{OS: "freestanding", Arch: "amd64", Sys: "none"}, false},
		{"uefi", buildconfig.Target{OS: "uefi", Arch: "amd64", Sys: "none"}, false},
		{"sys none", buildconfig.Target{OS: "linux", Arch: "amd64", Sys: "none"}, false},
		{"wasi", buildconfig.Target{OS: "wasi", Arch: "wasm32", Sys: "posix"}, false},
		{"isr", buildconfig.Target{OS: "linux", Arch: "amd64", Sys: "posix", BlockingExecutor: "isr"}, false},
		{"alloc-free-executor", buildconfig.Target{OS: "linux", Arch: "amd64", Sys: "posix", BlockingExecutor: "alloc-free-executor"}, false},
		{"single-thread-event-loop", buildconfig.Target{OS: "linux", Arch: "amd64", Sys: "posix", BlockingExecutor: "single-thread-event-loop"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := capabilityPlatform(c.target).Provides(opdef.CapTaskThreads).Provided; got != c.want {
				t.Errorf("%+v provides task.threads = %v, want %v", c.target, got, c.want)
			}
		})
	}
}

func TestWantsTaskCatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		target buildconfig.Target
		want   bool
	}{
		{"linux", buildconfig.Target{OS: "linux", Arch: "amd64", Sys: "posix"}, true},
		{"macos", buildconfig.Target{OS: "darwin", Arch: "arm64", Sys: "posix"}, true},
		{"wasi", buildconfig.Target{OS: "wasi", Arch: "wasm32", Sys: "posix"}, false},
		{"freestanding wasm", buildconfig.Target{OS: "freestanding", Arch: "wasm32", Sys: "none"}, false},
		{"freestanding x86_64", buildconfig.Target{OS: "freestanding", Arch: "amd64", Sys: "none"}, true},
		{"uefi x86_64", buildconfig.Target{OS: "uefi", Arch: "amd64", Sys: "none"}, true},
		{"freestanding aarch64", buildconfig.Target{OS: "freestanding", Arch: "arm64", Sys: "none"}, false},
		{"sys none x86_64", buildconfig.Target{OS: "linux", Arch: "amd64", Sys: "none"}, true},
		{"sys none riscv64", buildconfig.Target{OS: "linux", Arch: "riscv64", Sys: "none"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := wantsTaskCatch(c.target); got != c.want {
				t.Errorf("wantsTaskCatch(%+v) = %v, want %v", c.target, got, c.want)
			}
		})
	}
}
