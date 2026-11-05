package llgen

import _ "embed"

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

//go:embed runtime/rt_sys_weak.c
var runtimeSys string

//go:embed runtime/rt_signal_hosted.c
var runtimeSignal string

//go:embed runtime/rt_stack_guard_hosted.c
var runtimeStackGuard string

//go:embed runtime/rt_freestanding.h
var runtimeFreestandingHeader string

// capFiles are rt_core.c's KIGUMI_CAP_-guarded #include fragments, never a
// compile input of their own; rt_core.c's preprocessor picks which ones a target's capabilities pull in.
var capFiles = []RuntimeFile{
	{"rt_cap_host_shell.inc", runtimeCapHostShell},
	{"rt_cap_host_args.inc", runtimeCapHostArgs},
	{"rt_cap_host_files.inc", runtimeCapHostFiles},
	{"rt_cap_host_net.inc", runtimeCapHostNet},
	{"rt_cap_host_dl.inc", runtimeCapHostDl},
	{"rt_cap_host_entropy.inc", runtimeCapHostEntropy},
	{"rt_cap_host_signals.inc", runtimeCapHostSignals},
	{"rt_cap_host_stdio.inc", runtimeCapHostStdio},
	{"rt_cap_ffi_cstring.inc", runtimeCapFfiCstring},
	{"rt_cap_ffi_raw.inc", runtimeCapFfiRaw},
	{"rt_cap_ffi_pin.inc", runtimeCapFfiPin},
	{"rt_cap_ffi_handle_table.inc", runtimeCapFfiHandleTable},
	{"rt_cap_mem_secure_erase.inc", runtimeCapMemSecureErase},
	{"rt_cap_task_blocking_pool.inc", runtimeCapTaskBlockingPool},
	{"rt_cap_task_executor.inc", runtimeCapTaskExecutor},
}

// coreFragments are rt_core.c's other #include fragments, split by family so the entry file only
// keeps the state, init and std dispatch it must own.
var coreFragments = []RuntimeFile{
	{"rt_int128.inc", runtimeInt128},
	{"rt_std_text.inc", runtimeStdText},
	{"rt_display.inc", runtimeDisplay},
	{"rt_number.inc", runtimeNumber},
	{"rt_std_dispatch.inc", runtimeStdDispatch},
	{"rt_drop.inc", runtimeDrop},
	{"rt_iter.inc", runtimeIter},
	{"rt_builtin.inc", runtimeBuiltin},
	{"rt_utf8.inc", runtimeUtf8},
	{"rt_mem.inc", runtimeMem},
	{"rt_value.inc", runtimeValue},
	{"rt_call.inc", runtimeCall},
	{"rt_async.inc", runtimeAsync},
	{"rt_alloc.inc", runtimeAlloc},
	{"rt_rc.inc", runtimeRC},
	{"rt_unwind.inc", runtimeUnwind},
	{"rt_adt.inc", runtimeADT},
}

//go:embed runtime/rt_std_text.inc
var runtimeStdText string

//go:embed runtime/rt_int128.inc
var runtimeInt128 string

//go:embed runtime/rt_display.inc
var runtimeDisplay string

//go:embed runtime/rt_number.inc
var runtimeNumber string

//go:embed runtime/rt_std_dispatch.inc
var runtimeStdDispatch string

//go:embed runtime/rt_drop.inc
var runtimeDrop string

//go:embed runtime/rt_iter.inc
var runtimeIter string

//go:embed runtime/rt_builtin.inc
var runtimeBuiltin string

//go:embed runtime/rt_utf8.inc
var runtimeUtf8 string

//go:embed runtime/rt_mem.inc
var runtimeMem string

//go:embed runtime/rt_value.inc
var runtimeValue string

//go:embed runtime/rt_call.inc
var runtimeCall string

//go:embed runtime/rt_async.inc
var runtimeAsync string

//go:embed runtime/rt_alloc.inc
var runtimeAlloc string

//go:embed runtime/rt_rc.inc
var runtimeRC string

//go:embed runtime/rt_unwind.inc
var runtimeUnwind string

//go:embed runtime/rt_adt.inc
var runtimeADT string

//go:embed runtime/rt_cap_host_shell.inc
var runtimeCapHostShell string

//go:embed runtime/rt_cap_host_args.inc
var runtimeCapHostArgs string

//go:embed runtime/rt_cap_host_files.inc
var runtimeCapHostFiles string

//go:embed runtime/rt_cap_host_net.inc
var runtimeCapHostNet string

//go:embed runtime/rt_cap_host_dl.inc
var runtimeCapHostDl string

//go:embed runtime/rt_cap_host_entropy.inc
var runtimeCapHostEntropy string

//go:embed runtime/rt_cap_host_signals.inc
var runtimeCapHostSignals string

//go:embed runtime/rt_cap_host_stdio.inc
var runtimeCapHostStdio string

//go:embed runtime/rt_cap_ffi_cstring.inc
var runtimeCapFfiCstring string

//go:embed runtime/rt_cap_ffi_raw.inc
var runtimeCapFfiRaw string

//go:embed runtime/rt_cap_ffi_pin.inc
var runtimeCapFfiPin string

//go:embed runtime/rt_cap_ffi_handle_table.inc
var runtimeCapFfiHandleTable string

//go:embed runtime/rt_cap_mem_secure_erase.inc
var runtimeCapMemSecureErase string

//go:embed runtime/rt_cap_task_blocking_pool.inc
var runtimeCapTaskBlockingPool string

//go:embed runtime/rt_cap_task_executor.inc
var runtimeCapTaskExecutor string
