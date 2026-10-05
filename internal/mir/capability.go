package mir

// hostCapabilityOps are StdOpNames entries only os.Host, or a value from it,
// can reach; every other provider op has an OS-free rt_core.c implementation.
var hostCapabilityOps = map[string]bool{
	"HOST_SHELL": true, "HOST_ARGS": true, "HOST_FILES": true, "HOST_NET": true,
	"HOST_DL": true, "HOST_ENTROPY": true, "HOST_SIGNALS": true,
	"HOST_STDIN": true, "HOST_STDOUT": true, "HOST_STDERR": true,
	"ARGS_GET": true, "ARGS_LEN": true,
	"NET_HTTP": true, "NET_CONN_DROP": true, "NET_LISTENER_DROP": true,
	"DL_SYMBOL_CALL": true,
}

// A strict profile (freestanding, no default allocator) never provides any of these.
func (id StdOpId) NeedsHostCapability() bool { return hostCapabilityOps[StdOpNames[id]] }
