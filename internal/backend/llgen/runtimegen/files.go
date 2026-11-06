// Package runtimegen owns the C runtime's assets, its generators, and the
// layout and ABI fingerprint the runtime shares with the emitted module.
package runtimegen

import "slices"

// RuntimeFile is one C source or header of the runtime.
type RuntimeFile struct {
	Name, Src string
}

// freestandingOS have no bundled libc headers at all; a hosted "--sys
// none" target keeps its real OS headers (config.Target.Bare's doc).
var freestandingOS = map[string]bool{"freestanding": true, "uefi": true}

// posixSignalOS are the hosted OSes whose <signal.h> rt_signal_hosted.c needs;
// the compiler's mirreach.PosixSignalOS copies this list, and llgen's test pins them together.
var posixSignalOS = map[string]bool{"linux": true, "darwin": true, "freebsd": true, "netbsd": true, "openbsd": true}

// Files includes rt_signal_hosted.c only for hosted POSIX OSes, which alone have <signal.h>.
func Files(sys, os string) []RuntimeFile {
	files := []RuntimeFile{{"rt.h", runtimeHeader}, {"rt_internal.h", runtimeInternalHeader}, {"rt_std_ops.h", runtimeStdOps}, {"rt_core.c", runtimeCore}, {"rt_float.c", runtimeFloat}, {"rt_sys_weak.c", runtimeSys}, {"rt_fs.c", runtimeFS}}
	files = append(files, capFiles...)
	files = append(files, coreFragments...)
	if sys != "none" && posixSignalOS[os] {
		files = append(files, RuntimeFile{"rt_signal_hosted.c", runtimeSignal}, RuntimeFile{"rt_stack_guard_hosted.c", runtimeStackGuard})
	}
	if freestandingOS[os] {
		files = append(files, RuntimeFile{"rt_freestanding.h", runtimeFreestandingHeader})
	}
	return files
}

// Source is the whole runtime as one text, for tests that scan it.
func Source() string {
	out := runtimeHeader + runtimeInternalHeader + runtimeStdOps + runtimeCore + runtimeFloat + runtimeSys + runtimeFS
	for _, f := range append(slices.Clone(capFiles), coreFragments...) {
		out += f.Src
	}
	return out
}
