package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

func loadReportingFixture(t *testing.T) *driver.Module {
	t.Helper()
	std, _ := filepath.Abs("../../../std")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.kg"), []byte(`import test from std/test

test "ok one" {
    if 1 != 1 {
        fail test.failure("unreachable")
    }
}

test "bad one" {
    fail test.failure("boom")
}
`), 0o644)
	m, err := driver.LoadModule(dir, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Test: true})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestRunTestsReportingWithList(t *testing.T) {
	m := loadReportingFixture(t)
	var out, errOut bytes.Buffer
	code, err := driver.RunTestsReportingWith(m, driver.TestRunOptions{List: true}, &out, &errOut)
	if err != nil || code != 0 {
		t.Fatalf("list: code=%d err=%v stderr=%s", code, err, errOut.String())
	}
	if !strings.Contains(out.String(), `main::"ok one"`) || !strings.Contains(out.String(), `main::"bad one"`) {
		t.Fatalf("list output missing a stable id:\n%s", out.String())
	}
}

func TestRunTestsReportingWithFilterAndFailure(t *testing.T) {
	m := loadReportingFixture(t)
	var out, errOut bytes.Buffer
	code, err := driver.RunTestsReportingWith(m, driver.TestRunOptions{Filter: "bad"}, &out, &errOut)
	if err != nil {
		t.Fatalf("filter: err=%v", err)
	}
	if code != 1 {
		t.Fatalf("filter: code=%d, want 1 (the filtered test fails)", code)
	}
	if strings.Contains(out.String(), "ok one") {
		t.Fatalf("filter leaked the non-matching test:\n%s", out.String())
	}
}

func TestRunTestsReportingWithSuiteTimeoutSkipsRest(t *testing.T) {
	m := loadReportingFixture(t)
	var out, errOut bytes.Buffer
	code, err := driver.RunTestsReportingWith(m, driver.TestRunOptions{SuiteTimeout: time.Nanosecond}, &out, &errOut)
	if err != nil {
		t.Fatalf("suite timeout: err=%v", err)
	}
	if code != 1 {
		t.Fatalf("suite timeout: code=%d, want 1 (a run that skipped tests is not green)", code)
	}
	if strings.Count(out.String(), "SKIP ") != 2 {
		t.Fatalf("want both tests skipped:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "FAIL: 0 passed, 0 failed, 2 skipped") {
		t.Fatalf("summary must not say ok when the exit code is 1:\n%s", out.String())
	}
}
