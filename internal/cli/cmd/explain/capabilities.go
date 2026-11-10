package explain

import (
	"fmt"
	"os"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/artifact"
	"kigumi/internal/toolchain/driver"
)

func explainCapabilities(io session.IO, s session.Session, target string) error {
	if fp, ok := artifactFingerprint(target); ok {
		tgt, err := s.Target()
		if err != nil {
			return &exit.UsageError{Err: err}
		}
		// The sidecar is a file like any other, so it is only worth reading when this toolchain could have written it: a target or
		// ABI another build recorded is a stale artifact, not something to report as this toolchain's own capabilities.
		if reasons := driver.ToolchainMatch(fp, tgt); len(reasons) > 0 {
			return errors.NewErrf("artifact %s is stale: %s; rebuild it, or pass --target for an artifact built for another target", target, reasons[0])
		}
		fmt.Fprintf(io.Out, "artifact %s (target %s)\n", target, fp.Target)
		printCapabilities(io.Out, fp.CapabilityProvision, fp.CapabilityReasons)
		return nil
	}
	m, err := s.Load(target, false, io.Err)
	if err != nil {
		return err
	}
	a, err := driver.Analyze(m)
	if err != nil {
		return errors.WrapErr(err, "analyze module")
	}
	if a.Prog == nil {
		return &exit.UsageError{Err: errors.NewErr("module has diagnostics; fix them first")}
	}
	tgt, err := s.Target()
	if err != nil {
		return &exit.UsageError{Err: err}
	}
	printCapabilities(io.Out, driver.ProvidedCapabilities(tgt), driver.CapabilityReasons(a, tgt))
	return nil
}

func artifactFingerprint(path string) (artifact.Fingerprint, bool) {
	if strings.HasSuffix(path, ".kg") {
		return artifact.Fingerprint{}, false
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		return artifact.Fingerprint{}, false
	}
	fp, ok, err := artifact.ReadFingerprintSidecar(path)
	return fp, ok && err == nil
}
