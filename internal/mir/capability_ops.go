package mir

// opCapability omits ops every target provides without a capability.
var opCapability = map[string]Capability{
	"HOST_SHELL": CapHostShell, "HOST_ARGS": CapHostArgs, "HOST_FILES": CapHostFiles,
	"HOST_NET": CapHostNet, "HOST_DL": CapHostDL, "HOST_ENTROPY": CapHostEntropy,
	"HOST_SIGNALS": CapHostSignals,
	"HOST_STDIN":   CapHostStdio, "HOST_STDOUT": CapHostStdio, "HOST_STDERR": CapHostStdio,
	"ARGS_GET": CapHostArgs, "ARGS_LEN": CapHostArgs,
	"NET_HTTP": CapHostNet, "NET_CONN_DROP": CapHostNet, "NET_LISTENER_DROP": CapHostNet,
	"DL_SYMBOL_CALL": CapHostDL,
	"CSTRING_NEW":    CapFFICString, "CSTRING_PTR": CapFFICString, "CSTRING_DROP": CapFFICString,
	"PIN_NEW": CapFFIPin, "PIN_PTR": CapFFIPin, "PIN_WITH": CapFFIPin,
	"PIN_DROP": CapFFIPin, "PIN_RELEASE": CapFFIPin, "PIN_RECLAIM": CapFFIPin,
	"HANDLE_TABLE_NEW": CapFFIHandleTable, "HANDLE_TABLE_INSERT": CapFFIHandleTable,
	"HANDLE_TABLE_WITH": CapFFIHandleTable, "HANDLE_TABLE_REMOVE": CapFFIHandleTable,
	"HANDLE_TABLE_RECLAIM": CapFFIHandleTable, "HANDLE_TABLE_CLOSE": CapFFIHandleTable,
	"HANDLE_TABLE_DROP": CapFFIHandleTable, "HANDLE_RAW": CapFFIHandleTable, "HANDLE_DROP": CapFFIHandleTable,
	"BLOCKING_POOL_NEW": CapTaskBlocking, "BLOCKING_POOL_RUN": CapTaskBlocking,
	"FFI_BYTES_FROM": CapFFIRaw, "FFI_BYTES_PTR": CapFFIRaw,
	"FFI_STRING": CapFFIRaw, "FFI_STRING_CHECKED": CapFFIRaw, "FFI_STRING_LOSSY": CapFFIRaw,
	"TASK_CURRENT_EXECUTOR": CapTaskExecutor, "EXECUTOR_RUN_TO_COMPLETION": CapTaskExecutor,
	"TASK_SPAWN": CapTaskExecutor, "TASK_CANCEL": CapTaskExecutor, "TASK_CANCEL_REQUESTED": CapTaskExecutor,
	"TASK_THREAD_POOL_EXECUTOR": CapTaskThreads, "TASK_HANDOFF": CapTaskExecutor,
	"SECURE_ERASE": CapSecureErase, "SECURE_ERASE_RAW": CapSecureErase, "SECURE_ERASE_TRY": CapSecureErase,
}
