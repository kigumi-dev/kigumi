package driver_test

import (
	"bytes"
	"os/exec"
	"time"
)

// runFaultCmd runs a fault-injection child and retries one that died from a signal without writing
// anything: the sweeps start thousands of short-lived processes, and a loaded runner can kill one
// before it reports. A real crash reproduces on the retry and still reaches the caller. Each attempt
// needs its own Cmd: Go does not allow reusing one after Run.
func runFaultCmd(cmd *exec.Cmd) (int, string, string) {
	for attempt := 0; ; attempt++ {
		next := exec.Command(cmd.Path, cmd.Args[1:]...)
		next.Dir, next.Env, next.Stdin = cmd.Dir, cmd.Env, cmd.Stdin
		var stdout, stderr bytes.Buffer
		next.Stdout, next.Stderr = &stdout, &stderr
		code := exitCodeOf(next.Run())
		if code >= 0 || stdout.Len() > 0 || stderr.Len() > 0 || attempt == 100 {
			return code, stdout.String(), stderr.String()
		}
		// A just-written executable can still be open for writing by another
		// parallel test's build; retry ETXTBSY instead of reporting a crash.
		time.Sleep(10 * time.Millisecond)
	}
}
