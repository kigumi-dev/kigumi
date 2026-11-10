package vm_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kigumi/internal/backend/vm"
)

// A test's writes to stdout and stderr are replayed in the order it made them, so a run that captures both keeps the
// interleaving instead of collecting one stream after the other.
func TestRunTestsReportingKeepsWriteOrderAcrossStreams(t *testing.T) {
	prog := buildRunnableProgram(t, `import test from std/test

test "streams" {
    print "out1"
    eprint "err1"
    print "out2"
}
`)
	var both bytes.Buffer
	if _, err := vm.RunTestsReporting(prog, vm.TestRunOptions{}, &both, &both); err != nil {
		t.Fatal(err)
	}
	if got := both.String(); !strings.HasPrefix(got, "out1\nerr1\nout2\n") {
		t.Errorf("captured output = %q, want out1,err1,out2 in write order", got)
	}
}

// A timed-out test still reports what it wrote and the panic it raised before the interpreter was stopped: the timeout is
// a verdict about the deadline, not permission to drop the diagnosis.
func TestRunTestsReportingTimeoutReportsOutputAndPanic(t *testing.T) {
	prog := buildRunnableProgram(t, `import test from std/test
import time from std/time

pub fn boom() -> Int {
    let a: Array[Int] = Array.empty()
    a[0]
}

test "slow panic" {
    print "captured-before-deadline"
    time.sleep(200)
    print "captured-before-panic"
    let v = boom()
    print "${v}"
}
`)
	var out, errOut bytes.Buffer
	failed, err := vm.RunTestsReporting(prog, vm.TestRunOptions{PerTestTimeout: 20 * time.Millisecond}, &out, &errOut)
	if err != nil {
		t.Fatalf("timeout: err=%v", err)
	}
	if failed != 1 {
		t.Errorf("timeout: failed=%d, want 1", failed)
	}
	if !strings.Contains(out.String(), `TIME "slow panic"`) {
		t.Errorf("missing the TIME line:\n%s", out.String())
	}
	for _, want := range []string{"captured-before-deadline", "captured-before-panic"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("timed-out test lost %q:\nstdout %s\nstderr %s", want, out.String(), errOut.String())
		}
	}
	if !strings.Contains(errOut.String(), "panic: index out of range") {
		t.Errorf("timed-out test lost its panic:\nstderr %s", errOut.String())
	}
}

// A test reported as TIME must not land a side effect after the verdict: the effect is refused even though the
// interpreter is still unwinding.
func TestRunTestsReportingTimeoutRefusesLateSideEffects(t *testing.T) {
	prog := buildRunnableProgram(t, `import test from std/test
import time from std/time
import fs from std/fs

pub fn touch() -> Void! {
    let p = fs.Path.fromString("late.txt")?
    host.files().writeText(p, "written after timeout")?
}

test "late writer" {
    time.sleep(150)
    touch()?
    print "wrote"
}
`)
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	failed, err := vm.RunTestsReporting(prog, vm.TestRunOptions{Options: vm.Options{Cwd: dir}, PerTestTimeout: 20 * time.Millisecond}, &out, &errOut)
	if err != nil {
		t.Fatalf("timeout: err=%v", err)
	}
	if failed != 1 {
		t.Errorf("timeout: failed=%d, want 1", failed)
	}
	// Give the abandoned goroutine its full unwind window before looking.
	time.Sleep(300 * time.Millisecond)
	if _, statErr := os.Stat(filepath.Join(dir, "late.txt")); statErr == nil {
		t.Errorf("a timed-out test wrote late.txt after the verdict:\n%s%s", out.String(), errOut.String())
	}
}

// Every JSON event type carries a fixed key set: a zero counter or duration is written as 0 instead of being omitted.
func TestRunTestsReportingJSONHasFixedKeysPerType(t *testing.T) {
	prog := buildRunnableProgram(t, `print "no tests here"
`)
	var out, errOut bytes.Buffer
	if _, err := vm.RunTestsReporting(prog, vm.TestRunOptions{JSON: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"suiteStart": {"formatVersion", "type", "total"},
		"suiteEnd":   {"formatVersion", "type", "total", "passed", "failed", "skipped", "durationMs"},
	}
	saw := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("bad event line %q: %v", line, err)
		}
		typ, _ := ev["type"].(string)
		keys, ok := want[typ]
		if !ok {
			t.Fatalf("unexpected event type %q", typ)
		}
		saw[typ] = true
		if len(ev) != len(keys) {
			t.Errorf("%s keys = %v, want exactly %v", typ, keysOf(ev), keys)
		}
		for _, k := range keys {
			if _, present := ev[k]; !present {
				t.Errorf("%s is missing %q even at its zero value: %s", typ, k, line)
			}
		}
	}
	for typ := range want {
		if !saw[typ] {
			t.Errorf("no %s event:\n%s", typ, out.String())
		}
	}
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
