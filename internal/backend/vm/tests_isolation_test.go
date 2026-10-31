package vm_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
)

const staticIsolationSrc = `import test from std/test

static mut counter: i32 = 0

print "entry"

test "first bump" {
    unsafe { counter = counter + 1 }
    unsafe { test.expectEq(counter, 1)? }
}

test "second bump" {
    unsafe { counter = counter + 1 }
    unsafe { test.expectEq(counter, 1)? }
}
`

func TestRunTestsResetsStaticMutBetweenTests(t *testing.T) {
	prog := buildRunnableProgram(t, staticIsolationSrc)
	var out, errOut bytes.Buffer
	failed, err := vm.RunTestsReporting(prog, vm.TestRunOptions{}, &out, &errOut)
	if err != nil || failed != 0 {
		t.Fatalf("failed=%d err=%v\nout:\n%s\nerr:\n%s", failed, err, out.String(), errOut.String())
	}
	out.Reset()
	failed, err = vm.RunTestsReporting(prog, vm.TestRunOptions{Filter: "second"}, &out, &errOut)
	if err != nil || failed != 0 {
		t.Fatalf("filtered run depends on the skipped test: failed=%d err=%v\n%s", failed, err, out.String())
	}
}

func TestRunTestsLegacyResetsStaticMutBetweenTests(t *testing.T) {
	prog := buildRunnableProgram(t, staticIsolationSrc)
	var out, errOut bytes.Buffer
	failed, err := vm.RunTests(prog, vm.Options{}, &out, &errOut)
	if err != nil || failed != 0 {
		t.Fatalf("failed=%d err=%v\n%s", failed, err, out.String())
	}
}

func TestRunTestsReportingJSONWrapsTestOutput(t *testing.T) {
	prog := buildRunnableProgram(t, `import test from std/test

print "entry"

test "talker" {
    print "hello from a test"
}
`)
	var out, errOut bytes.Buffer
	if _, err := vm.RunTestsReporting(prog, vm.TestRunOptions{JSON: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var sawOutput bool
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var ev struct {
			Type    string `json:"type"`
			ID      string `json:"id"`
			Stream  string `json:"stream"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("raw line between events: %q", line)
		}
		if ev.Type == "testOutput" {
			sawOutput = true
			if ev.Stream != "stdout" || ev.Message != "hello from a test\n" || !strings.Contains(ev.ID, "talker") {
				t.Errorf("unexpected output event %+v", ev)
			}
		}
	}
	if !sawOutput {
		t.Errorf("no testOutput event:\n%s", out.String())
	}
}

func TestRunTestsReportingContinuesPastUnsupportedTest(t *testing.T) {
	prog := buildRunnableProgram(t, `import test from std/test

extern(C) {
    fn abs(x: i32) -> i32
}

print "entry"

test "first" {
    test.expectEq(1, 1)?
}

test "foreign" {
    test.expectEq(unsafe { abs(-3) }, 3)?
}

test "last" {
    test.expectEq(2, 2)?
}
`)
	var out, errOut bytes.Buffer
	failed, err := vm.RunTestsReporting(prog, vm.TestRunOptions{}, &out, &errOut)
	if err != nil || failed != 1 {
		t.Fatalf("failed=%d err=%v\n%s", failed, err, out.String())
	}
	for _, want := range []string{`ok   "first"`, `FAIL "foreign"`, `ok   "last"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
	if !strings.Contains(errOut.String(), "foreign function abs is not supported by the VM") {
		t.Errorf("reason missing from stderr:\n%s", errOut.String())
	}
}
