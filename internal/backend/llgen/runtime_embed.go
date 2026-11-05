package llgen

import (
	"slices"

	mirreach "kigumi/internal/compiler/mir/reach"
)

// RuntimeFile is one C source or header of the runtime.
type RuntimeFile struct {
	Name, Src string
}

// freestandingOS have no bundled libc headers at all; a hosted "--sys
// none" target keeps its real OS headers (config.Target.Bare's doc).
var freestandingOS = map[string]bool{"freestanding": true, "uefi": true}

// RuntimeFiles includes rt_signal_hosted.c only for hosted POSIX OSes, which alone have <signal.h>.
func RuntimeFiles(sys, os string) []RuntimeFile {
	files := []RuntimeFile{{"rt.h", runtimeHeader}, {"rt_internal.h", runtimeInternalHeader}, {"rt_std_ops.h", runtimeStdOps}, {"rt_core.c", runtimeCore}, {"rt_float.c", runtimeFloat}, {"rt_sys_weak.c", runtimeSys}}
	files = append(files, capFiles...)
	files = append(files, coreFragments...)
	if sys != "none" && mirreach.PosixSignalOS(os) {
		files = append(files, RuntimeFile{"rt_signal_hosted.c", runtimeSignal}, RuntimeFile{"rt_stack_guard_hosted.c", runtimeStackGuard})
	}
	if freestandingOS[os] {
		files = append(files, RuntimeFile{"rt_freestanding.h", runtimeFreestandingHeader})
	}
	return files
}

// RuntimeSource is the whole runtime as one text, for tests that scan it.
func RuntimeSource() string {
	out := runtimeHeader + runtimeInternalHeader + runtimeStdOps + runtimeCore + runtimeFloat + runtimeSys
	for _, f := range append(slices.Clone(capFiles), coreFragments...) {
		out += f.Src
	}
	return out
}
