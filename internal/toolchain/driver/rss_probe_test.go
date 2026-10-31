package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// probedRSS runs body in a process of its own and reports its output and peak resident size in KB. A child's ru_maxrss also counts the peak of the process that forked it, so the parent test starts a probe process (fresh, small) and the probe starts the process that runs body.
func probedRSS(t *testing.T, body func() string) (string, int64) {
	out, rss, _ := probedUsage(t, body)
	return out, rss
}

// probedUsage is probedRSS that also reports the CPU time the process running body used, which a loaded machine stretches far less than wall time.
func probedUsage(t *testing.T, body func() string) (string, int64, time.Duration) {
	t.Helper()
	rerun := func(role string) *exec.Cmd {
		pattern := "^" + strings.ReplaceAll(t.Name(), "/", "$/^") + "$"
		cmd := exec.Command(os.Args[0], "-test.run="+pattern, "-test.count=1")
		cmd.Env = append(os.Environ(), "KIGUMI_RSS_ROLE="+role)
		return cmd
	}
	switch os.Getenv("KIGUMI_RSS_ROLE") {
	case "run":
		fmt.Printf("rss-out %q\n", body())
		t.SkipNow()
	case "probe":
		cmd := rerun("run")
		report, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("run: %v\n%s", err, report)
		}
		var out string
		if i := bytes.Index(report, []byte("rss-out ")); i < 0 {
			t.Fatalf("no output\n%s", report)
		} else if _, err := fmt.Sscanf(string(report[i:]), "rss-out %q", &out); err != nil {
			t.Fatalf("output: %v\n%s", err, report)
		}
		fmt.Printf("rss-probe %d %d %q\n", cmd.ProcessState.SysUsage().(*syscall.Rusage).Maxrss, (cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()).Milliseconds(), out)
		t.SkipNow()
	}
	if runtime.GOOS != "linux" {
		return body(), 0, 0
	}
	report, err := rerun("probe").CombinedOutput()
	var rss, cpuMS int64
	var out string
	if i := bytes.Index(report, []byte("rss-probe ")); err != nil || i < 0 {
		t.Fatalf("probe: %v\n%s", err, report)
	} else if _, err := fmt.Sscanf(string(report[i:]), "rss-probe %d %d %q", &rss, &cpuMS, &out); err != nil {
		t.Fatalf("probe output: %v\n%s", err, report)
	}
	return out, rss, time.Duration(cpuMS) * time.Millisecond
}
