package driver

import "testing"

func TestWantsThreadRuntime(t *testing.T) {
	t.Parallel()
	hosted := Target{OS: "linux", Arch: "amd64", Sys: "posix"}
	cases := []struct {
		name   string
		target Target
		uses   bool
		want   bool
	}{
		{"hosted without a pool", hosted, false, false},
		{"hosted with a pool", hosted, true, true},
		{"freestanding", Target{OS: "freestanding", Arch: "amd64", Sys: "none"}, true, false},
		{"uefi", Target{OS: "uefi", Arch: "amd64", Sys: "none"}, true, false},
		{"sys none", Target{OS: "linux", Arch: "amd64", Sys: "none"}, true, false},
		{"isr", Target{OS: "linux", Arch: "amd64", Sys: "posix", BlockingExecutor: "isr"}, true, false},
		{"alloc-free-executor", Target{OS: "linux", Arch: "amd64", Sys: "posix", BlockingExecutor: "alloc-free-executor"}, true, false},
		{"single-thread-event-loop", Target{OS: "linux", Arch: "amd64", Sys: "posix", BlockingExecutor: "single-thread-event-loop"}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := wantsThreadRuntime(c.target, c.uses); got != c.want {
				t.Errorf("wantsThreadRuntime(%+v, %v) = %v, want %v", c.target, c.uses, got, c.want)
			}
		})
	}
}
