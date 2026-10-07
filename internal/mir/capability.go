package mir

import (
	"sort"
	"strings"
)

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
	CapFFIRaw         Capability = "ffi.raw"
	CapFFIPin         Capability = "ffi.pin"
	CapFFIHandleTable Capability = "ffi.handle-table"
	CapTaskBlocking   Capability = "task.blocking-pool"
	CapTaskExecutor   Capability = "task.executor"
	CapTaskThreads    Capability = "task.threads"
	CapSecureErase    Capability = "mem.secure-erase"
)

// Single source of capability names for the rt_core.c guards, resolver, summary and artifact.
var vocabulary = []Capability{
	CapHostShell, CapHostArgs, CapHostFiles, CapHostNet, CapHostDL,
	CapHostEntropy, CapHostSignals, CapHostStdio,
	CapFFICString, CapFFIRaw, CapFFIPin, CapFFIHandleTable,
	CapTaskBlocking, CapTaskExecutor, CapTaskThreads, CapSecureErase,
}

var capabilityNeeds = map[Capability][]Capability{
	CapTaskThreads: {CapTaskExecutor},
}

func Vocabulary() []Capability { return append([]Capability(nil), vocabulary...) }

func (c Capability) Needs() []Capability { return capabilityNeeds[c] }

func (c Capability) HasSource() bool { return c != CapTaskThreads }

func (c Capability) SourceFile() string {
	return "rt_cap_" + strings.NewReplacer(".", "_", "-", "_").Replace(string(c)) + ".c.inc"
}

func (id StdOpId) Capability() (Capability, bool) {
	c, ok := opCapability[StdOpNames[id]]
	return c, ok
}

func (c Capability) Macro() string {
	up := strings.ToUpper(string(c))
	up = strings.ReplaceAll(up, ".", "_")
	up = strings.ReplaceAll(up, "-", "_")
	return "KIGUMI_CAP_" + up
}

func closure(seen map[Capability]bool) {
	for changed := true; changed; {
		changed = false
		for c := range seen {
			for _, need := range c.Needs() {
				if !seen[need] {
					seen[need] = true
					changed = true
				}
			}
		}
	}
}

func Capabilities(ops []StdOpId) []string {
	seen := map[Capability]bool{}
	for _, op := range ops {
		if c, ok := op.Capability(); ok {
			seen[c] = true
		}
	}
	closure(seen)
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, string(c))
	}
	sort.Strings(out)
	return out
}

func CapabilityMacros(ops []StdOpId) []string {
	names := Capabilities(ops)
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = Capability(n).Macro()
	}
	return out
}
