package driver

import (
	"testing"

	"kigumi/internal/compiler/mir"
)

func TestTargetProvidesWorkerThreads(t *testing.T) {
	t.Parallel()
	hosted := Target{OS: "linux", Arch: "amd64", Sys: "posix"}
	cases := []struct {
		name   string
		target Target
		want   bool
	}{
		{"hosted", hosted, true},
		{"zero target is the host", Target{}, true},
		{"freestanding", Target{OS: "freestanding", Arch: "amd64", Sys: "none"}, false},
		{"uefi", Target{OS: "uefi", Arch: "amd64", Sys: "none"}, false},
		{"sys none", Target{OS: "linux", Arch: "amd64", Sys: "none"}, false},
		{"wasi", Target{OS: "wasi", Arch: "wasm32", Sys: "posix"}, false},
		{"isr", Target{OS: "linux", Arch: "amd64", Sys: "posix", BlockingExecutor: "isr"}, false},
		{"alloc-free-executor", Target{OS: "linux", Arch: "amd64", Sys: "posix", BlockingExecutor: "alloc-free-executor"}, false},
		{"single-thread-event-loop", Target{OS: "linux", Arch: "amd64", Sys: "posix", BlockingExecutor: "single-thread-event-loop"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := c.target.CapabilityPlatform().Provides(mir.CapTaskThreads).Provided; got != c.want {
				t.Errorf("%+v provides task.threads = %v, want %v", c.target, got, c.want)
			}
		})
	}
}

func TestWantsTaskCatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		target Target
		want   bool
	}{
		{"linux", Target{OS: "linux", Arch: "amd64", Sys: "posix"}, true},
		{"macos", Target{OS: "darwin", Arch: "arm64", Sys: "posix"}, true},
		{"wasi", Target{OS: "wasi", Arch: "wasm32", Sys: "posix"}, false},
		{"freestanding wasm", Target{OS: "freestanding", Arch: "wasm32", Sys: "none"}, false},
		{"freestanding x86_64", Target{OS: "freestanding", Arch: "amd64", Sys: "none"}, true},
		{"uefi x86_64", Target{OS: "uefi", Arch: "amd64", Sys: "none"}, true},
		{"freestanding aarch64", Target{OS: "freestanding", Arch: "arm64", Sys: "none"}, false},
		{"sys none x86_64", Target{OS: "linux", Arch: "amd64", Sys: "none"}, true},
		{"sys none riscv64", Target{OS: "linux", Arch: "riscv64", Sys: "none"}, false},
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
