package vm_test

import (
	"bytes"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
)

// A module root that is also the only package must not repeat its name: `iso/iso::` reads as two different things when
// it is one, and the help text promises `<package>::<name>`.
func TestTestIDString(t *testing.T) {
	cases := []vm.TestID{
		{Module: "iso", Package: "iso", Name: `"a bump"`},
		{Module: "iso", Package: "iso/sub", Name: `"a bump"`},
		{Module: "iso", Package: "other", Name: `"a bump"`},
		{Module: "", Package: "main", Name: `"a bump"`},
		{Module: "", Package: "", Name: `"a bump"`},
	}
	want := []string{
		`iso::"a bump"`,
		`iso/sub::"a bump"`,
		`iso/other::"a bump"`,
		`main::"a bump"`,
		`"a bump"`,
	}
	for i, id := range cases {
		if got := id.String(); got != want[i] {
			t.Errorf("TestID%+v = %q, want %q", id, got, want[i])
		}
	}
}

// A --filter matches a test id, and the name is part of it, so filtering by name keeps working after the module segment
// is dropped.
func TestRunTestsReportingFilterByName(t *testing.T) {
	prog := buildRunnableProgram(t, `import test from std/test

test "first bump" {
    print "first"
}

test "second bump" {
    print "second"
}
`)
	var out, errOut bytes.Buffer
	failed, err := vm.RunTestsReporting(prog, vm.TestRunOptions{Module: "app", Filter: "second bump"}, &out, &errOut)
	if err != nil || failed != 0 {
		t.Fatalf("filter: failed=%d err=%v\n%s%s", failed, err, out.String(), errOut.String())
	}
	if strings.Contains(out.String(), "first") {
		t.Errorf("filter leaked the non-matching test:\n%s", out.String())
	}
	if !strings.Contains(out.String(), `ok   "second bump"`) {
		t.Errorf("filter dropped the matching test:\n%s", out.String())
	}
}

// A print the process cannot write is a failed run, not a silent success: print/eprint have no error channel to report
// it through.
func TestRunReportsLostOutput(t *testing.T) {
	prog := buildRunnableProgram(t, `fn main() {
    print "lost"
}
`)
	var errOut bytes.Buffer
	code, err := vm.Run(prog, vm.Options{}, failingWriter{}, &errOut)
	if err != nil {
		t.Fatalf("run: err=%v", err)
	}
	if code != 1 {
		t.Errorf("exit = %d, want 1: %s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "write stdout") || !strings.Contains(errOut.String(), "no space left") {
		t.Errorf("stderr = %q, want the lost output reported", errOut.String())
	}
}

// failingWriter fails every write the way a full device does.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errNoSpace }

type noSpaceError struct{}

func (noSpaceError) Error() string { return "no space left on device" }

var errNoSpace = noSpaceError{}
