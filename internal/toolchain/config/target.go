package config

import (
	"fmt"
	"strings"

	"kigumi/internal/lang/target"
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

// parsedTriple is the one structural reading of a triple: the target facts a
// Target is minted from, and the raw components ABISegment and CompileTriple
// read, so no method has to split the triple a second time.
type parsedTriple struct {
	// parts are the triple's dash-separated components as written.
	parts    []string
	arch, os string
	// abi is the ABI segment of a well-formed <arch>-<os>-<abi> triple; ""
	// when the triple names no ABI segment.
	abi string
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
	p := parsedTriple{parts: parts, arch: arch, os: os, abiKnown: true}
	switch {
	case len(parts) == 3:
		// Exactly one component after <arch>-<os> is the ABI; it may still be
		// unrecognized, which only marks the target unverified.
		p.abi, p.abiKnown = parts[2], knownABI[parts[1]][parts[2]]
	case len(parts) > 3:
		// A longer triple has no single ABI segment: whatever follows the OS
		// is unverified and cannot pick a C library.
		p.abiKnown = false
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

// TargetFromTriple mints a target from a zig-style triple. Triple, OS, Arch,
// Sys and Tier all come from one reading of the string, so the four facts can
// never disagree with each other or with the triple. It is the only way to
// get a non-host Target; the host's own facts are HostTarget's, since an
// empty triple is a marker for it, not a triple. sys is "" (the triple
// decides), "posix" or "none"; a freestanding or UEFI triple has no
// operating system, so "none" is forced and "posix" is rejected.
func TargetFromTriple(triple, sys string) (Target, error) {
	p, err := parseTripleStructure(triple)
	if err != nil {
		return Target{}, err
	}
	t := Target{Triple: triple, OS: p.os, Arch: p.arch, Sys: "posix"}
	if !p.abiKnown {
		t.Tier = "unverified"
	}
	return resolveSys(normalizeTarget(t), sys, triple)
}

// ParseTarget reads a triple; "" is o's host. A named triple always goes
// through TargetFromTriple; the host keeps its explicit default.
func ParseTarget(o CompilerOptions, triple, sys string) (Target, error) {
	if triple != "" {
		return TargetFromTriple(triple, sys)
	}
	return resolveSys(normalizeTarget(o.HostOrAmbient()), sys, triple)
}

// WithTriple returns t retargeted at triple, the way resolveLinkage moves a
// host target to the C library triple it links. The policy fields pass
// through unchanged, while Triple, OS, Arch, Sys and Tier are re-derived
// together, so the old target's facts can never survive beside a new Triple.
// It returns t unchanged when triple does not parse.
func (t Target) WithTriple(triple string) (Target, error) {
	// A --sys none build stays bare; otherwise the triple decides the layer,
	// so retargeting a hosted target at a freestanding triple cannot leave a
	// posix layer beside a target that has no operating system.
	sys := ""
	if t.Sys == "none" {
		sys = "none"
	}
	next, err := TargetFromTriple(triple, sys)
	if err != nil {
		return t, err
	}
	next.Allocator = t.Allocator
	next.Profile = t.Profile
	next.BlockingExecutor = t.BlockingExecutor
	next.StrictStack = t.StrictStack
	next.StackBudgetKiB = t.StackBudgetKiB
	return next, nil
}

// Freestanding targets have no operating system: the build is an archive
// with a `kigumi_main` entry for the embedder's startup code.
func (t Target) Freestanding() bool { return t.OS == "freestanding" || t.OS == "uefi" }

// COFF reports a PE/COFF object format (UEFI today); -nostdlib leaves no CRT, so rt_core.c defines `_fltused`.
func (t Target) COFF() bool { return t.OS == "uefi" }

// Bare targets get Freestanding's archive output even on a hosted OS, since `--sys none`
// means no OS services; a host-OS bare archive still uses that OS's own headers and ABI.
func (t Target) Bare() bool { return t.Freestanding() || t.Sys == "none" }

// Layout is the width-dependent and C-ABI view of the target for the
// checker, the backend and the header importer; the triple's ABI segment
// picks the C library variant (Windows msvc makes long double a double).
func (t Target) Layout() target.Layout {
	return target.ForOSABI(t.OS, t.Arch, t.ABISegment())
}

// ABISegment is the triple's optional third component ("gnu", "msvc",
// "musl", ...); "" leaves the platform's default C library ABI and is also
// what an unverified triple with extra components gets, since it names no
// single ABI segment.
func (t Target) ABISegment() string {
	p, err := parseTripleStructure(t.Triple)
	if err != nil {
		return ""
	}
	return p.abi
}

// StaticLibc reports a triple whose ABI segment selects the static musl C
// library, the only one kigumi links statically.
func (t Target) StaticLibc() bool { return strings.HasPrefix(t.ABISegment(), "musl") }

// Name is the target as a report records it: the triple when the user named
// one, else the host's platform names. The host's form is a label, not a
// parsable triple, because HostTarget deliberately leaves Triple empty.
func (t Target) Name() string {
	if t.Triple != "" {
		return t.Triple
	}
	return t.OS + "-" + t.Arch
}

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

// CompileTriple is the triple handed to the C compiler: as given, except that zig names the OS-less "none" OS "freestanding"
// and has no 32-bit ARM Windows ABI in ARM mode, which UEFI on arm needs as thumb.
func (t Target) CompileTriple() string {
	p, err := parseTripleStructure(t.Triple)
	if err != nil {
		return t.Triple
	}
	parts := append([]string(nil), p.parts...)
	if parts[1] == "none" {
		parts[1] = "freestanding"
	}
	if p.os == "uefi" && p.arch == "arm" {
		parts[0] = "thumb"
	}
	return strings.Join(parts, "-")
}
