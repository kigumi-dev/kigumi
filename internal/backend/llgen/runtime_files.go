package llgen

import (
	_ "embed"
	"slices"

	mirreach "kigumi/internal/compiler/mir/reach"
)

//go:embed runtime/rt.h
var runtimeHeader string

//go:embed runtime/rt_internal.h
var runtimeInternalHeader string

//go:embed runtime/rt_std_ops.h
var runtimeStdOps string

//go:embed runtime/rt_core.c
var runtimeCore string

//go:embed runtime/rt_float.c
var runtimeFloat string

//go:embed runtime/rt_sys.c
var runtimeSys string

//go:embed runtime/rt_signal.c
var runtimeSignal string

//go:embed runtime/rt_stack_guard.c
var runtimeStackGuard string

//go:embed runtime/rt_freestanding.h
var runtimeFreestandingHeader string

// capFiles are rt_core.c's KIGUMI_CAP_-guarded #include fragments, never a
// compile input of their own; rt_core.c's preprocessor picks which ones a target's capabilities pull in.
var capFiles = []RuntimeFile{
	{"rt_cap_host_shell.c.inc", runtimeCapHostShell},
	{"rt_cap_host_args.c.inc", runtimeCapHostArgs},
	{"rt_cap_host_files.c.inc", runtimeCapHostFiles},
	{"rt_cap_host_net.c.inc", runtimeCapHostNet},
	{"rt_cap_host_dl.c.inc", runtimeCapHostDl},
	{"rt_cap_host_entropy.c.inc", runtimeCapHostEntropy},
	{"rt_cap_host_signals.c.inc", runtimeCapHostSignals},
	{"rt_cap_host_stdio.c.inc", runtimeCapHostStdio},
	{"rt_cap_ffi_cstring.c.inc", runtimeCapFfiCstring},
	{"rt_cap_ffi_raw.c.inc", runtimeCapFfiRaw},
	{"rt_cap_ffi_pin.c.inc", runtimeCapFfiPin},
	{"rt_cap_ffi_handle_table.c.inc", runtimeCapFfiHandleTable},
	{"rt_cap_mem_secure_erase.c.inc", runtimeCapMemSecureErase},
	{"rt_cap_task_blocking_pool.c.inc", runtimeCapTaskBlockingPool},
	{"rt_cap_task_executor.c.inc", runtimeCapTaskExecutor},
}

// coreFragments are rt_core.c's other #include fragments, built only when a program needs them.
var coreFragments = []RuntimeFile{{"rt_int128.c.inc", runtimeInt128}, {"rt_try_text.c.inc", runtimeTryText}}

//go:embed runtime/rt_try_text.c.inc
var runtimeTryText string

//go:embed runtime/rt_int128.c.inc
var runtimeInt128 string

//go:embed runtime/rt_cap_host_shell.c.inc
var runtimeCapHostShell string

//go:embed runtime/rt_cap_host_args.c.inc
var runtimeCapHostArgs string

//go:embed runtime/rt_cap_host_files.c.inc
var runtimeCapHostFiles string

//go:embed runtime/rt_cap_host_net.c.inc
var runtimeCapHostNet string

//go:embed runtime/rt_cap_host_dl.c.inc
var runtimeCapHostDl string

//go:embed runtime/rt_cap_host_entropy.c.inc
var runtimeCapHostEntropy string

//go:embed runtime/rt_cap_host_signals.c.inc
var runtimeCapHostSignals string

//go:embed runtime/rt_cap_host_stdio.c.inc
var runtimeCapHostStdio string

//go:embed runtime/rt_cap_ffi_cstring.c.inc
var runtimeCapFfiCstring string

//go:embed runtime/rt_cap_ffi_raw.c.inc
var runtimeCapFfiRaw string

//go:embed runtime/rt_cap_ffi_pin.c.inc
var runtimeCapFfiPin string

//go:embed runtime/rt_cap_ffi_handle_table.c.inc
var runtimeCapFfiHandleTable string

//go:embed runtime/rt_cap_mem_secure_erase.c.inc
var runtimeCapMemSecureErase string

//go:embed runtime/rt_cap_task_blocking_pool.c.inc
var runtimeCapTaskBlockingPool string

//go:embed runtime/rt_cap_task_executor.c.inc
var runtimeCapTaskExecutor string

// RuntimeFile is one C source or header of the runtime.
type RuntimeFile struct {
	Name, Src string
}

// freestandingOS have no bundled libc headers at all; a hosted "--sys
// none" target keeps its real OS headers (config.Target.Bare's doc).
var freestandingOS = map[string]bool{"freestanding": true, "uefi": true}

// RuntimeFiles includes rt_signal.c only for hosted POSIX OSes, which alone have <signal.h>.
func RuntimeFiles(sys, os string) []RuntimeFile {
	files := []RuntimeFile{{"rt.h", runtimeHeader}, {"rt_internal.h", runtimeInternalHeader}, {"rt_std_ops.h", runtimeStdOps}, {"rt_core.c", runtimeCore}, {"rt_float.c", runtimeFloat}, {"rt_sys.c", runtimeSys}}
	files = append(files, capFiles...)
	files = append(files, coreFragments...)
	if sys != "none" && mirreach.PosixSignalOS(os) {
		files = append(files, RuntimeFile{"rt_signal.c", runtimeSignal}, RuntimeFile{"rt_stack_guard.c", runtimeStackGuard})
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
