// Package target describes what the compiler builds for: the pointer width
// that fixes usize and isize, the C scalar model that fixes layouts and
// signatures at the C boundary, and the calling convention that fixes how
// integers cross it. It is immutable and passed explicitly; nothing reads it
// from the environment.
package target

import "runtime"

// Layout is everything the compiler needs to know about a target that is
// not written in the source.
type Layout struct {
	OS, Arch string
	// PtrBits is the pointer width: the width of usize, isize and raw pointers.
	PtrBits int
	// BigEndian reports the target's byte order; false is little-endian.
	BigEndian bool
	// C is the target's C scalar model: the sizes and alignments that decide
	// a layout(C) record and an extern(C) signature.
	C CModel
	// ABI is the target's C calling convention and aggregate rule.
	ABI ABI
}

// Default is the layout a zero-value caller gets: 64-bit little-endian x86
// running Linux, which is what kigumi's own emitters assume when no target
// is given.
func Default() Layout { return ForArch("linux", "amd64") }

// ForArch gives the layout of an OS / architecture pair under the target's
// default C library ABI. It panics for an architecture kigumi doesn't know;
// callers reach this only through Host(), since ParseTarget rejects an
// unknown --target arch before it gets here.
func ForArch(os, arch string) Layout { return ForOSABI(os, arch, "") }

// ForOSABI is ForArch with the triple's ABI segment (the optional third
// component: "gnu", "msvc", "musl", ...). It changes the C model only where
// the C library ABI does: the Microsoft ABI makes long double a plain
// double, while mingw keeps the x87 one on x86.
func ForOSABI(os, arch, abi string) Layout {
	f, ok := archFactsTable[arch]
	if !ok {
		panic("target: unknown architecture " + arch)
	}
	l := Layout{OS: os, Arch: arch, PtrBits: f.ptrBits, BigEndian: f.bigEndian, C: f.c, ABI: f.abi}
	applyPlatform(&l, os, arch, abi)
	return l
}

// Host is the layout of the machine kigumi runs on.
func Host() Layout {
	arch := runtime.GOARCH
	if arch == "wasm" {
		arch = "wasm32"
	}
	return ForArch(runtime.GOOS, arch)
}
