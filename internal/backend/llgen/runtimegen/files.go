// Package runtimegen owns the C runtime's assets, its generators, and the
// layout and ABI fingerprint the runtime shares with the emitted module.
package runtimegen

import "slices"

// RuntimeFile is one source or header of the runtime: a C translation unit or
// header the C compiler takes, or a Zig source `zig build-obj` compiles.
type RuntimeFile struct {
	Name, Src string
}

// freestandingOS have no bundled libc headers at all; a hosted "--sys
// none" target keeps its real OS headers (config.Target.Bare's doc).
var freestandingOS = map[string]bool{"freestanding": true, "uefi": true}

// posixSignalOS are the hosted OSes whose <signal.h> rt_signal_hosted.c needs;
// the compiler's mirreach.PosixSignalOS copies this list, and llgen's test pins them together.
var posixSignalOS = map[string]bool{"linux": true, "darwin": true, "freebsd": true, "netbsd": true, "openbsd": true}

// stackGuardOS have either the guard-page implementation or the OS hash seed
// in rt_stack_guard_hosted.c; the other POSIX OSes keep the weak defaults.
var stackGuardOS = map[string]bool{"linux": true, "darwin": true, "freebsd": true}

// Files includes rt_signal_hosted.c only for hosted POSIX OSes, which alone
// have <signal.h>; rt_stack_guard_hosted.c only where it has a body or seed;
// and rt_fs.c only for hosted targets.
func Files(sys, os string) []RuntimeFile {
	files := []RuntimeFile{{"rt.h", runtimeHeader}, {"rt_std_ops.h", runtimeStdOps}, {"rt_core.c", runtimeCore}, {"rt_sys_weak.c", runtimeSys}}
	if sys != "none" && !freestandingOS[os] {
		files = append(files, RuntimeFile{"rt_fs.c", runtimeFS})
	}
	files = append(files, capFiles...)
	files = append(files, coreFragments...)
	if sys != "none" && posixSignalOS[os] {
		files = append(files, RuntimeFile{"rt_signal_hosted.c", runtimeSignal})
	}
	if sys != "none" && stackGuardOS[os] {
		files = append(files, RuntimeFile{"rt_stack_guard_hosted.c", runtimeStackGuard})
	}
	if freestandingOS[os] {
		files = append(files, RuntimeFile{"rt_freestanding.h", runtimeFreestandingHeader})
	}
	return files
}

// ZigFiles are the runtime's Zig translation units. They are not C compile
// inputs: the build materialises them and runs `zig build-obj` once, then
// links the resulting object beside the C ones. Names keep the `zig/` prefix
// so the relative imports inside the module resolve as written.
func ZigFiles() []RuntimeFile {
	return []RuntimeFile{
		{"zig/rt_float.zig", zigRtFloat},
		{"zig/float/bignum.zig", zigFloatBignum},
		{"zig/float/dragon4.zig", zigFloatDragon4},
		{"zig/float/algorithm_m.zig", zigFloatAlgorithmM},
	}
}

// Source is the C runtime as one text, for tests that scan it; the Zig
// translation units are separate and have no C preprocessor to scan.
func Source() string {
	out := runtimeHeader + runtimeStdOps + runtimeCore + runtimeSys + runtimeFS
	for _, f := range append(slices.Clone(capFiles), coreFragments...) {
		out += f.Src
	}
	return out
}
