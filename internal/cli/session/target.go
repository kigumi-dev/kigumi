package session

import (
	"fmt"

	buildconfig "kigumi/internal/toolchain/config"
)

// Target resolves --target and --sys with the allocator, failure, blocking and stack profiles.
func (s Session) Target() (buildconfig.Target, error) {
	t, err := buildconfig.ParseTarget(CompilerOptions(), s.Triple, s.Sys)
	if err != nil {
		return t, err
	}
	switch s.Allocator {
	case "", "general":
	case "none":
		t.Allocator = "none"
	default:
		return t, fmt.Errorf("allocator %q: want general or none", s.Allocator)
	}
	switch s.Profile {
	case "", "fatal-permitted":
	case "recoverable-only":
		t.Profile = "recoverable-only"
	default:
		return t, fmt.Errorf("profile %q: want fatal-permitted or recoverable-only", s.Profile)
	}
	switch s.BlockingExecutor {
	case "", "hosted":
	case "isr", "alloc-free-executor", "single-thread-event-loop":
		t.BlockingExecutor = s.BlockingExecutor
	default:
		return t, fmt.Errorf("blocking-executor %q: want hosted, isr, alloc-free-executor or single-thread-event-loop", s.BlockingExecutor)
	}
	switch s.StackProfile {
	case "", "hosted-growable":
	case "strict":
		t.StrictStack = true
	default:
		return t, fmt.Errorf("stack-profile %q: want hosted-growable or strict", s.StackProfile)
	}
	t.StackBudgetKiB = s.StackBudgetKiB
	return t, nil
}
