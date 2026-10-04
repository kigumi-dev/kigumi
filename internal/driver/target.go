package driver

import (
	"fmt"
	"runtime"
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
	// Tier is "" or "unverified", for a triple whose ABI segment
	// ParseTarget does not recognize.
	Tier string
}

var targetArch = map[string]string{"x86_64": "amd64", "aarch64": "arm64", "riscv64": "riscv64", "wasm32": "wasm32", "x86": "386", "arm": "arm"}

var targetOS = map[string]string{"linux": "linux", "macos": "darwin", "windows": "windows", "freebsd": "freebsd", "netbsd": "netbsd", "openbsd": "openbsd", "wasi": "wasi", "freestanding": "freestanding", "uefi": "uefi", "none": "freestanding"}

// knownABI lists the ABI segment (the triple's optional third component)
// each OS is tested against; an OS absent here accepts only no ABI segment.
var knownABI = map[string]map[string]bool{
	"linux":   {"musl": true, "gnu": true},
	"windows": {"gnu": true, "msvc": true},
}

// HostTarget is the machine kigumi runs on.
func HostTarget() Target {
	return Target{OS: runtime.GOOS, Arch: runtime.GOARCH, Sys: "posix"}
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

// ParseTarget reads a triple; "" is the host.
func ParseTarget(triple, sys string) (Target, error) {
	t := HostTarget()
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

// COFF reports a PE/COFF object format (UEFI today), which needs the
// _fltused marker rt_core.c defines since -nostdlib leaves no CRT to.
func (t Target) COFF() bool { return t.OS == "uefi" }

// Bare targets get Freestanding's archive output even on a hosted OS, since `--sys none`
// means no OS services; a host-OS bare archive still uses that OS's own headers and ABI.
func (t Target) Bare() bool { return t.Freestanding() || t.Sys == "none" }

// Layout is the width-dependent view of the target for the checker and
// the backend.
func (t Target) Layout() target.Layout { return target.ForArch(t.OS, t.Arch) }

// NoDefaultAllocator reports the `--allocator none` profile.
func (t Target) NoDefaultAllocator() bool { return t.Allocator == "none" }

// compileTriple is the triple as given; build.go supplies rt_freestanding.h's own declarations for freestanding/UEFI since compileTriple no longer substitutes a musl triple.
func (t Target) compileTriple() string { return t.Triple }
