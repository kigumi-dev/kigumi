package mir

import (
	"fmt"

	opdef "kigumi/internal/compiler/mir/ops"
)

type Platform struct {
	OS string
	// Bare is true when the build has no OS services: freestanding, uefi or --sys none.
	Bare bool
	// NoLibc is true when the OS ships no libc headers at all (freestanding, uefi).
	NoLibc bool
	// Allocator "none" withholds every heap-backed capability.
	Allocator string
	// RecoverableOnly withholds the heap-backed capabilities too.
	RecoverableOnly bool
	ThreadProfile   string
}

func PosixSignalOS(os string) bool {
	switch os {
	case "linux", "darwin", "freebsd", "netbsd", "openbsd":
		return true
	}
	return false
}

type Provision struct {
	Provided bool
	Reason   string
}

var provisionRules = map[opdef.Capability]func(Platform) string{
	opdef.CapHostStdio:   needsOS,
	opdef.CapHostArgs:    needsOS,
	opdef.CapHostFiles:   needsOS,
	opdef.CapHostEntropy: needsOS,
	opdef.CapHostShell:   needsOSExcept("wasi", "has no process spawning"),
	opdef.CapHostNet:     needsOSExcept("wasi", "has no sockets"),
	opdef.CapHostDL:      needsOSExcept("wasi", "has no dynamic loader"),
	opdef.CapHostSignals: func(p Platform) string {
		if why := needsOS(p); why != "" {
			return why
		}
		if !PosixSignalOS(p.OS) {
			return fmt.Sprintf("%s has no POSIX signals", p.OS)
		}
		return ""
	},
	opdef.CapTaskBlocking: func(p Platform) string {
		if p.NoLibc {
			return fmt.Sprintf("the %s runtime does not include the blocking pool", p.OS)
		}
		return ""
	},
	opdef.CapTaskThreads: func(p Platform) string {
		switch {
		case p.Bare:
			return "worker threads need an operating system"
		case p.ThreadProfile != "":
			return fmt.Sprintf("profile %s forbids worker threads", p.ThreadProfile)
		case p.OS == "wasi":
			return "wasi has no worker threads"
		}
		return ""
	},
}

func needsOS(p Platform) string {
	if p.Bare {
		return "freestanding, uefi and --sys none builds have no operating system services"
	}
	return ""
}

func needsOSExcept(os, what string) func(Platform) string {
	return func(p Platform) string {
		if why := needsOS(p); why != "" {
			return why
		}
		if p.OS == os {
			return os + " " + what
		}
		return ""
	}
}

// A capability is withheld when one it needs is.
func (p Platform) Provides(c opdef.Capability) Provision {
	if c.HeapBacked() {
		if why := p.heapWithheld(); why != "" {
			return Provision{Reason: why}
		}
	}
	if rule := provisionRules[c]; rule != nil {
		if why := rule(p); why != "" {
			return Provision{Reason: why}
		}
	}
	for _, need := range c.Needs() {
		if got := p.Provides(need); !got.Provided {
			return Provision{Reason: fmt.Sprintf("needs %s: %s", need, got.Reason)}
		}
	}
	return Provision{Provided: true}
}

func (p Platform) heapWithheld() string {
	switch {
	case p.Allocator == "none":
		return "its runtime state is heap-allocated outside any allocator block, and this build has no default allocator (--allocator none)"
	case p.RecoverableOnly:
		return "its runtime state is heap-allocated by an operation that terminates on a refused allocation (--profile recoverable-only)"
	}
	return ""
}
