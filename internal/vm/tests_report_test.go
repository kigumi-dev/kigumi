package vm_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"kigumi/internal/mir"
	"kigumi/internal/vm"
)

const testsReportSrc = `import test from std/test

test "first passes" {
    if 1 != 1 {
        fail test.failure("unreachable")
    }
}

test "second fails" {
    fail test.failure("boom")
}
`

// buildRunnableProgram also runs plan-rc: mir.Build alone leaves OpShare's Copy/retain choice unresolved, unlike checkSource's scalar-only callers.
func buildRunnableProgram(t *testing.T, src string) *mir.Program {
	t.Helper()
	res := checkSource(t, src)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}); err != nil {
		t.Fatalf("plan-rc: %v", err)
	}
	return prog
}

func buildTestsProgram(t *testing.T) *mir.Program {
	return buildRunnableProgram(t, testsReportSrc)
}

func TestRunTestsReportingList(t *testing.T) {
	prog := buildTestsProgram(t)
	var out, errOut bytes.Buffer
	failed, err := vm.RunTestsReporting(prog, vm.TestRunOptions{Module: "app", List: true}, &out, &errOut)
	if err != nil || failed != 0 {
		t.Fatalf("list: failed=%d err=%v", failed, err)
	}
	for _, want := range []string{`app/main::"first passes"`, `app/main::"second fails"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("--list missing %q in:\n%s", want, out.String())
		}
	}
}

func TestRunTestsReportingFilter(t *testing.T) {
	prog := buildTestsProgram(t)
	var out, errOut bytes.Buffer
	failed, err := vm.RunTestsReporting(prog, vm.TestRunOptions{Module: "app", Filter: "first"}, &out, &errOut)
	if err != nil {
		t.Fatalf("filter: err=%v", err)
	}
	if failed != 0 {
		t.Errorf("filter should only select the passing test, failed=%d", failed)
	}
	if strings.Contains(out.String(), "second fails") {
		t.Errorf("filter leaked the non-matching test:\n%s", out.String())
	}
	if !strings.Contains(out.String(), `ok   "first passes"`) {
		t.Errorf("filter: missing the ok line:\n%s", out.String())
	}
}

func TestRunTestsReportingJSON(t *testing.T) {
	prog := buildTestsProgram(t)
	var out, errOut bytes.Buffer
	failed, err := vm.RunTestsReporting(prog, vm.TestRunOptions{JSON: true}, &out, &errOut)
	if err != nil || failed != 1 {
		t.Fatalf("json: failed=%d err=%v", failed, err)
	}
	var sawFail, sawEnd bool
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var ev struct {
			FormatVersion int    `json:"formatVersion"`
			Type          string `json:"type"`
			Failed        int    `json:"failed"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("bad event line %q: %v", line, err)
		}
		if ev.FormatVersion != vm.TestEventFormatVersion {
			t.Errorf("event %q: formatVersion=%d, want %d", ev.Type, ev.FormatVersion, vm.TestEventFormatVersion)
		}
		if ev.Type == "testFail" {
			sawFail = true
		}
		if ev.Type == "suiteEnd" {
			sawEnd = true
			if ev.Failed != 1 {
				t.Errorf("suiteEnd.failed = %d, want 1", ev.Failed)
			}
		}
	}
	if !sawFail || !sawEnd {
		t.Errorf("missing testFail or suiteEnd event:\n%s", out.String())
	}
}

// TestRunTestsReportingPanicCarriesSite checks a panicking test's testFail event names the panic's own file:line, not just the test declaration's.
func TestRunTestsReportingPanicCarriesSite(t *testing.T) {
	src := `import test from std/test

test "divides by zero" {
    let x = 1
    let y = 0
    let z = x / y
    if z == z {
        fail test.failure("unreachable")
    }
}
`
	prog := buildRunnableProgram(t, src)
	var out, errOut bytes.Buffer
	failed, err := vm.RunTestsReporting(prog, vm.TestRunOptions{JSON: true}, &out, &errOut)
	if err != nil || failed != 1 {
		t.Fatalf("failed=%d err=%v\nstderr=%s", failed, err, errOut.String())
	}
	var sawFail bool
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var ev struct {
			Type string `json:"type"`
			Line int    `json:"line"`
			File string `json:"file"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("bad event line %q: %v", line, err)
		}
		if ev.Type != "testFail" {
			continue
		}
		sawFail = true
		// Line 6 ("let z = x / y") is the panic site, not line 3's test declaration.
		if ev.Line != 6 {
			t.Errorf("testFail.line = %d, want 6 (the division, not the test declaration)", ev.Line)
		}
		if !strings.HasSuffix(ev.File, ".kg") {
			t.Errorf("testFail.file = %q, want a .kg path", ev.File)
		}
	}
	if !sawFail {
		t.Fatal("no testFail event")
	}
}

// TestRunTestsReportingTimeoutDoesNotRaceNextTest guards against the abandoned timed-out goroutine racing the next test on the shared out/errOut writers.
func TestRunTestsReportingTimeoutDoesNotRaceNextTest(t *testing.T) {
	src := `import test from std/test

test "spins and prints" {
    let mut x = 0
    for i in 0..2000000000 {
        print "spin ${i}"
        x = x + i
    }
}

test "quick" {
    if 1 != 1 {
        fail test.failure("unreachable")
    }
}
`
	prog := buildRunnableProgram(t, src)
	for i := 0; i < 5; i++ {
		var out, errOut bytes.Buffer
		if _, err := vm.RunTestsReporting(prog, vm.TestRunOptions{PerTestTimeout: 20 * time.Millisecond}, &out, &errOut); err != nil {
			t.Fatalf("run %d: err=%v", i, err)
		}
	}
}

func TestRunTestsReportingPerTestTimeout(t *testing.T) {
	src := `import test from std/test

test "spins forever" {
    let mut x = 0
    for i in 0..2000000000 {
        x = x + i
    }
}
`
	prog := buildRunnableProgram(t, src)
	var out, errOut bytes.Buffer
	failed, err := vm.RunTestsReporting(prog, vm.TestRunOptions{PerTestTimeout: 50 * time.Millisecond}, &out, &errOut)
	if err != nil {
		t.Fatalf("timeout: err=%v", err)
	}
	if failed != 1 {
		t.Fatalf("timeout: failed=%d, want 1", failed)
	}
	if !strings.Contains(out.String(), "TIME ") {
		t.Errorf("timeout: missing TIME line:\n%s", out.String())
	}
}

func TestRunTestsReportingStdPanicReportsTheTestSite(t *testing.T) {
	src := `import test from std/test

test "asserts" {
    test.assert(false, "boom")
}
`
	prog := buildRunnableProgram(t, src)
	var out, errOut bytes.Buffer
	failed, err := vm.RunTestsReporting(prog, vm.TestRunOptions{JSON: true}, &out, &errOut)
	if err != nil || failed != 1 {
		t.Fatalf("failed=%d err=%v\nstderr=%s", failed, err, errOut.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var ev struct {
			Type string `json:"type"`
			Line int    `json:"line"`
			File string `json:"file"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("bad event line %q: %v", line, err)
		}
		if ev.Type == "testFail" && (ev.Line != 3 || strings.Contains(ev.File, "std/")) {
			t.Errorf("testFail at %s:%d, want the test block at line 3 outside std", ev.File, ev.Line)
		}
	}
}
