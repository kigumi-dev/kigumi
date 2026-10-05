package mir

import "sort"

// Capability is a reachable std op's runtime feature, named for a build's
// reason graph (kigumi build's summary, kigumi explain) instead of a StdOpNames id.
type Capability string

const (
	CapHostShell      Capability = "host.shell"
	CapHostArgs       Capability = "host.args"
	CapHostFiles      Capability = "host.files"
	CapHostNet        Capability = "host.net"
	CapHostDL         Capability = "host.dl"
	CapHostEntropy    Capability = "host.entropy"
	CapHostSignals    Capability = "host.signals"
	CapHostStdio      Capability = "host.stdio"
	CapFFICString     Capability = "ffi.cstring"
	CapFFIPin         Capability = "ffi.pin"
	CapFFIHandleTable Capability = "ffi.handle-table"
	CapTaskBlocking   Capability = "task.blocking-pool"
)

// hostOnlyCapabilities are reachable only through os.Host; the ffi/task
// capabilities above have an OS-free rt_core.c implementation instead.
var hostOnlyCapabilities = map[Capability]bool{
	CapHostShell: true, CapHostArgs: true, CapHostFiles: true, CapHostNet: true,
	CapHostDL: true, CapHostEntropy: true, CapHostSignals: true, CapHostStdio: true,
}

func (c Capability) HostOnly() bool { return hostOnlyCapabilities[c] }

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
}

func (id StdOpId) Capability() (Capability, bool) {
	c, ok := opCapability[StdOpNames[id]]
	return c, ok
}

// A strict profile (freestanding, no default allocator) never provides a host-only capability.
func (id StdOpId) NeedsHostCapability() bool {
	c, ok := id.Capability()
	return ok && c.HostOnly()
}

func Capabilities(ops []StdOpId) []string {
	seen := map[Capability]bool{}
	for _, op := range ops {
		if c, ok := op.Capability(); ok {
			seen[c] = true
		}
	}
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, string(c))
	}
	sort.Strings(out)
	return out
}
