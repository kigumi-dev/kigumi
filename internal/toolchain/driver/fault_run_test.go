package driver_test

import (
	"bytes"
	"os/exec"
)

// runFaultCmd runs a fault-injection child and retries one that died from a signal without writing
// anything: the sweeps start thousands of short-lived processes, and a loaded runner can kill one
// before it reports. A real crash reproduces on the retry and still reaches the caller.
func runFaultCmd(cmd *exec.Cmd) (int, string, string) {
	for attempt := 0; ; attempt++ {
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		code := exitCodeOf(cmd.Run())
		if code >= 0 || stdout.Len() > 0 || stderr.Len() > 0 || attempt == 2 {
			return code, stdout.String(), stderr.String()
		}
	}
}
