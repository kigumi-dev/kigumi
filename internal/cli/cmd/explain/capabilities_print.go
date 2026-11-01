package explain

import (
	"fmt"
	"io"
	"strings"

	"kigumi/internal/toolchain/artifact"
	"kigumi/internal/toolchain/driver"
)

func printCapabilities(out io.Writer, provision []artifact.CapabilityVerdict, reasons []driver.CapabilityReason) {
	if len(reasons) == 0 {
		fmt.Fprint(out, "no named capability in this closed vocabulary\n")
		return
	}
	verdict := map[string]artifact.CapabilityVerdict{}
	for _, v := range provision {
		verdict[v.Capability] = v
	}
	last := ""
	for _, cr := range reasons {
		if cr.Capability != last {
			last = cr.Capability
			fmt.Fprintf(out, "%s: %s\n", cr.Capability, provisionLine(cr, verdict))
		}
		fmt.Fprintf(out, "  %s (%s)\n", reasonSubject(cr), cr.RootKind)
		printSites(out, cr)
	}
}

func printSites(out io.Writer, cr driver.CapabilityReason) {
	if len(cr.Sites) == 0 {
		fmt.Fprintf(out, "    %s\n", strings.Join(cr.Path, " -> "))
		return
	}
	for i, site := range cr.Sites {
		line := strings.Join(site.Path, " -> ")
		if i > 0 {
			line += " (" + site.RootKind + ")"
		}
		if site.Calls > 1 {
			line += fmt.Sprintf(" x%d", site.Calls)
		}
		fmt.Fprintf(out, "    %s\n", line)
	}
}

func provisionLine(cr driver.CapabilityReason, verdict map[string]artifact.CapabilityVerdict) string {
	if v, ok := verdict[cr.Capability]; ok {
		cr.Provided, cr.Why = v.Provided, v.Why
	}
	if cr.Provided {
		return "provided"
	}
	return "withheld, " + cr.Why
}

func reasonSubject(cr driver.CapabilityReason) string {
	if cr.RequiredBy != "" {
		return "needed by " + cr.RequiredBy
	}
	return cr.Op
}
