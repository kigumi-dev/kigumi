package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/toolchain/driver"
)

const panicFormSweep = 24

// panicFormCases are the std functions that are a Kigumi body over their try form; setup runs before the arming eprint, call after it.
// ticks is how many ticks the try form refuses before the call completes, the same on both backends.
var panicFormCases = []struct {
	name, setup, call string
	ticks             int
}{
	{"bytes_slice", "let b = Bytes.zeros(4)", "let _ = b.slice(1, 3)", 2},
	{"bytes_zeros", "", "let _ = Bytes.zeros(4)", 2},
	{"bytes_fill", "", "let _ = Bytes.fill(7, 4)", 2},
	{"bytes_from_array", "let a = Array.of[u8](1, 2, 3)", "let _ = Bytes.fromArray(&a)", 2},
	{"bytes_concat", "let a = Bytes.zeros(2)\n    let b = Bytes.zeros(2)", "let _ = a.concat(b)", 2},
	{"string_slice_bytes", "let s = \"hello world\".concat(\"!\")", "let _ = s.sliceBytes(1, 4)", 2},
	{"array_clone", "let a = Array.of(1, 2, 3)", "let _ = a.clone()", 2},
	{"array_take", "let a = Array.of(1, 2, 3)", "let _ = a.take(2)", 2},
	{"array_reverse", "let a = Array.of(1, 2, 3)", "let _ = a.reverse()", 2},
	{"array_concat", "let a = Array.of(1, 2, 3)\n    let b = Array.of(7, 8)", "let _ = a.concat(b)", 2},
	{"array_append_all", "let mut a = Array.of(1, 2, 3, 4)\n    let b = Array.of(4, 5, 6, 7)", "a.appendAll(b)", 1},
	{"shared_new", "", "let _ = alloc.Shared.new(41)", 3},
	{"shared_get", "let s = alloc.Shared.new(41)", "let _ = s.get()", 0},
	{"shared_downgrade", "let s = alloc.Shared.new(41)", "let _ = s.downgrade()", 1},
	{"box_new", "", "let _ = alloc.Box.new(5)", 2},
}

func panicFormSource(setup, call string) string {
	if setup != "" {
		setup = "    " + setup + "\n"
	}
	return "import alloc from std/alloc\n\nfn run() {\n" + setup + "    eprint \"go\"\n    " + call + "\n}\n\nrun()\n"
}

func classifyPanicFormRun(exit int, stdout, stderr string) byte {
	switch {
	case exit == 0 && stdout == "":
		return 'O'
	case exit == 2 && stdout == "" && strings.HasSuffix(stderr, "panic: out of memory\n"):
		return 'P'
	}
	return '?'
}

// TestFaultInjectionPanickingFormsPanicWhenATickIsRefused checks that every body over its try form turns a refused tick into the out-of-memory panic on both backends, and that the ticks after the call are untouched.
func TestFaultInjectionPanickingFormsPanicWhenATickIsRefused(t *testing.T) {
	t.Parallel()
	hasCC := testCompiler().CCompiler() != nil
	for _, c := range panicFormCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			src := panicFormSource(c.setup, c.call)
			for backend, classes := range map[string]func(*testing.T, string) []byte{"native": panicFormNativeClasses, "vm": panicFormVMClasses} {
				if backend == "native" && !hasCC {
					continue
				}
				got := classes(t, src)
				if want := regexp.MustCompile("^P{" + strconv.Itoa(c.ticks) + "}O+$"); !want.Match(got) {
					t.Errorf("%s outcomes %s do not match %s", backend, got, want)
				}
			}
		})
	}
}

func panicFormNativeClasses(t *testing.T, src string) []byte {
	t.Helper()
	exe := buildFaultInjectable(t, src)
	var classes []byte
	for n := 1; n <= panicFormSweep; n++ {
		cmd := exec.Command(exe)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_ARM=1", "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		classes = append(classes, classifyPanicFormRun(exitCodeOf(cmd.Run()), stdout.String(), stderr.String()))
	}
	return classes
}

func panicFormVMClasses(t *testing.T, src string) []byte {
	t.Helper()
	a, err := driver.Analyze(loadTryConstructorModule(t, src))
	if err != nil || a.Res.HasErrors() {
		t.Fatalf("analyze: %v\n%s", err, a.Res.Render())
	}
	var classes []byte
	for n := 1; n <= panicFormSweep; n++ {
		var out, errOut bytes.Buffer
		code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: int64(n), FaultArmOnEprint: true}, &out, &errOut)
		if err != nil {
			t.Fatalf("N=%d: %v", n, err)
		}
		classes = append(classes, classifyPanicFormRun(code, out.String(), errOut.String()))
	}
	return classes
}
