package explain

import (
	"fmt"
	"io"
	"os"
	"strings"

	"kigumi/internal/cli/shared"
	"kigumi/internal/cliutil"
	"kigumi/internal/driver"
	"kigumi/internal/errors"

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
		fmt.Fprintf(out, "    %s\n", strings.Join(cr.Path, " -> "))
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
