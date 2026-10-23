package driver

import (
	"fmt"
	"strings"

	"kigumi/internal/target"
)

// Target is what kigumi build --target compiles for: a zig-style triple; OS and Arch use the platform-file names, Sys picks the runtime layer.
type Target struct {
	Triple string
	OS     string
	Arch   string
	// Sys is "posix" or "none"; a freestanding OS forces "none".
	Sys string
	// Allocator is "" (the general allocator) or "none": the build has no
	// default allocator and the checker enforces that from the entry.
	Allocator string
	// Profile is "" or "recoverable-only" (no reachable path may
	// terminate on a refused allocation); independent of Allocator.
	Profile string
	// BlockingExecutor is "" (hosted default: warns on a direct-blocking op reached from async) or a strict
	// profile (isr, alloc-free-executor, single-thread-event-loop) that rejects it; independent of Profile and Allocator.
	BlockingExecutor string
	// Tier is "" or "unverified", for a triple whose ABI segment
	// ParseTarget does not recognize.
	Tier string
	// StrictStack requires every stack root to have a bounded call chain;
	// the zero value is hosted-growable, which only reports the summary.
	StrictStack bool
	// StackBudgetKiB is the stack capacity in KiB a strict profile's
	// bounded roots must fit under; 0 only requires boundedness.
	StackBudgetKiB int
}

var targetArch = map[string]string{"x86_64": "amd64", "aarch64": "arm64", "riscv64": "riscv64", "wasm32": "wasm32", "x86": "386", "arm": "arm"}

var targetOS = map[string]string{"linux": "linux", "macos": "darwin", "windows": "windows", "freebsd": "freebsd", "netbsd": "netbsd", "openbsd": "openbsd", "wasi": "wasi", "freestanding": "freestanding", "uefi": "uefi", "none": "freestanding"}

// knownABI lists the ABI segment (the triple's optional third component)
// each OS is tested against; an OS absent here accepts only no ABI segment.
var knownABI = map[string]map[string]bool{
	"linux":   {"musl": true, "gnu": true},
	"windows": {"gnu": true, "msvc": true},
}

// Unverified reports a triple whose ABI ParseTarget could not check against
// a known list; the build passes it to the external compiler as given.
func (t Target) Unverified() bool { return t.Tier == "unverified" }

// parsedTriple is ParseTarget's structural reading of a triple, before
// normalize and resolve apply cross-field invariants.
type parsedTriple struct {
	arch, os string
	// abiKnown false means an unrecognized ABI segment; still builds, Unverified.
	abiKnown bool
}

func parseTripleStructure(triple string) (parsedTriple, error) {
	parts := strings.Split(triple, "-")
	if len(parts) < 2 {
		return parsedTriple{}, fmt.Errorf("target %q: want <arch>-<os>[-<abi>]", triple)
	}
	arch, ok := targetArch[parts[0]]
	if !ok {
		return parsedTriple{}, fmt.Errorf("target %q: unknown architecture %s", triple, parts[0])
	}
	os, ok := targetOS[parts[1]]
	if !ok {
		return parsedTriple{}, fmt.Errorf("target %q: unknown OS %s", triple, parts[1])
	}
	if os == "uefi" && arch == "wasm32" {
		return parsedTriple{}, fmt.Errorf("target %q: wasm32 has no UEFI ABI", triple)
	}
	p := parsedTriple{arch: arch, os: os, abiKnown: true}
	if len(parts) >= 3 {
		p.abiKnown = len(parts) == 3 && knownABI[parts[1]][parts[2]]
	}
	return p, nil
}

// normalizeTarget applies the one invariant ParseTarget must not let a
// later --sys override: freestanding and UEFI have no OS, so Sys is none.
func normalizeTarget(t Target) Target {
	if t.Freestanding() {
		t.Sys = "none"
	}
	return t
}

// resolveSys rejects --sys posix against a freestanding/UEFI target.
func resolveSys(t Target, sys, triple string) (Target, error) {
	if sys == "" {
		return t, nil
	}
	if sys != "posix" && sys != "none" {
		return t, fmt.Errorf("sys %q: want posix or none", sys)
	}
	if sys == "posix" && t.Freestanding() {
		return t, fmt.Errorf("target %q has no operating system; --sys posix contradicts it", triple)
	}
	t.Sys = sys
	return t, nil
}

// ParseTarget reads a triple; "" is o's host.
func ParseTarget(o CompilerOptions, triple, sys string) (Target, error) {
	t := o.HostOrAmbient()
	if triple != "" {
		p, err := parseTripleStructure(triple)
		if err != nil {
			return t, err
		}
		t = Target{Triple: triple, OS: p.os, Arch: p.arch, Sys: "posix"}
		if !p.abiKnown {
			t.Tier = "unverified"
		}
	}
	t = normalizeTarget(t)
	return resolveSys(t, sys, triple)
}

// Freestanding targets have no operating system: the build is an archive
// with a `kigumi_main` entry for the embedder's startup code.
func (t Target) Freestanding() bool { return t.OS == "freestanding" || t.OS == "uefi" }

// COFF reports a PE/COFF object format (UEFI today); -nostdlib leaves no CRT, so rt_core.c defines `_fltused`.
func (t Target) COFF() bool { return t.OS == "uefi" }

// Bare targets get Freestanding's archive output even on a hosted OS, since `--sys none`
// means no OS services; a host-OS bare archive still uses that OS's own headers and ABI.
func (t Target) Bare() bool { return t.Freestanding() || t.Sys == "none" }

// Layout is the width-dependent view of the target for the checker and
// the backend.
func (t Target) Layout() target.Layout { return target.ForArch(t.OS, t.Arch) }

// NoDefaultAllocator reports the `--allocator none` profile.
func (t Target) NoDefaultAllocator() bool { return t.Allocator == "none" }

// RecoverableOnly reports the `--profile recoverable-only` profile.
func (t Target) RecoverableOnly() bool { return t.Profile == "recoverable-only" }

// RestrictsAllocation reports a profile that makes a planned copy a diagnostic.
func (t Target) RestrictsAllocation() bool { return t.NoDefaultAllocator() || t.RecoverableOnly() }

// strictBlockingProfiles names every `--blocking-executor` value that rejects a direct-blocking op reached
// from async instead of warning; the hosted default ("") is not in this set.
var strictBlockingProfiles = map[string]bool{
	"isr": true, "alloc-free-executor": true, "single-thread-event-loop": true,
}

// StrictBlocking reports a `--blocking-executor` profile that rejects direct blocking inside an async body instead of warning.
func (t Target) StrictBlocking() bool { return strictBlockingProfiles[t.BlockingExecutor] }

// compileTriple is the triple handed to the C compiler: as given, except that zig names the OS-less "none" OS "freestanding"
// and has no 32-bit ARM Windows ABI in ARM mode, which UEFI on arm needs as thumb.
func (t Target) compileTriple() string {
	parts := strings.Split(t.Triple, "-")
	if len(parts) < 2 {
		return t.Triple
	}
	if parts[1] == "none" {
		parts[1] = "freestanding"
	}
	if t.OS == "uefi" && t.Arch == "arm" {
		parts[0] = "thumb"
	}
	return strings.Join(parts, "-")
}
