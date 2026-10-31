package explain

import (
	"fmt"
	"io"
	"os"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"

	"github.com/spf13/cobra"
)

func explainCapabilities(cmd *cobra.Command, target string) error {
	if fp, ok := artifactFingerprint(target); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "artifact %s (target %s)\n", target, fp.Target)
		printCapabilities(cmd.OutOrStdout(), fp.CapabilityProvision, fp.CapabilityReasons)
		return nil
	}
	m, err := shared.Load(cmd, target, false, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	a, err := driver.Analyze(m)
	if err != nil {
		return errors.WrapErr(err, "analyze module")
	}
	if a.Prog == nil {
		return &cliutil.UsageError{Err: errors.NewErr("module has diagnostics; fix them first")}
	}
	tgt, err := shared.Target(cmd)
	if err != nil {
		return &cliutil.UsageError{Err: err}
	}
	printCapabilities(cmd.OutOrStdout(), driver.ProvidedCapabilities(tgt), driver.CapabilityReasons(a, tgt))
	return nil
}

func artifactFingerprint(path string) (driver.ArtifactFingerprint, bool) {
	if strings.HasSuffix(path, ".kg") {
		return driver.ArtifactFingerprint{}, false
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		return driver.ArtifactFingerprint{}, false
	}
	fp, ok, err := driver.ReadFingerprintSidecar(path)
	return fp, ok && err == nil
}

func printCapabilities(out io.Writer, provision []driver.CapabilityVerdict, reasons []driver.CapabilityReason) {
	if len(reasons) == 0 {
		fmt.Fprint(out, "no named capability in this closed vocabulary\n")
		return
	}
	verdict := map[string]driver.CapabilityVerdict{}
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

func provisionLine(cr driver.CapabilityReason, verdict map[string]driver.CapabilityVerdict) string {
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
