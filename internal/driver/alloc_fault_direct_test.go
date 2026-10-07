package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/vm"
)

// directBoxSrc has exactly one allocation the VM can see: the box the
// direct record needs to reach a type parameter.
const directBoxSrc = `type Point = {
    pub x Int
    pub y Int
}

fn pick[T](v: T) -> T { v }

fn run() -> Int {
    let p = Point { x: 1, y: 2 }
    pick(p).x
}

print "ok=${run()}"
`

const directBoxSweep = 120

func checkBoxOutcome(t *testing.T, backend string, n, exit int, out, errOut string) bool {
	t.Helper()
	switch exit {
	case 0:
		return false
	case 2:
		if !strings.Contains(errOut, "out of memory") {
			t.Errorf("%s N=%d: exit 2 without an out-of-memory panic: %q", backend, n, errOut)
		}
		return true
	}
	t.Errorf("%s N=%d: exit %d (want 0 or 2), stdout=%q stderr=%q", backend, n, exit, out, errOut)
	return false
}

func TestFaultInjectionDirectBoxNative(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	m := loadTryConstructorModule(t, directBoxSrc)
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{
		CFlags: []string{"-DKIGUMI_FAULT_INJECT", "-DKIGUMI_RC_CHECK"},
	})
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	refused := false
	for n := 1; n <= directBoxSweep; n++ {
		cmd := exec.Command(exe)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		refused = checkBoxOutcome(t, "native", n, exitCodeOf(cmd.Run()), out.String(), errOut.String()) || refused
	}
	if !refused {
		t.Errorf("native: no N in 1..%d refused an allocation", directBoxSweep)
	}
}

// The VM counts the crossing and nothing else in this program, so the
// first injected failure is the box itself, and both backends end the same way.
func TestFaultInjectionDirectBoxVM(t *testing.T) {
	t.Parallel()
	m := loadTryConstructorModule(t, directBoxSrc)
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatal(a.Res.Render())
	}
	for n := 1; n <= 8; n++ {
		var out, errOut bytes.Buffer
		code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: int64(n)}, &out, &errOut)
		if err != nil {
			t.Fatalf("N=%d: run error: %v", n, err)
		}
		refused := checkBoxOutcome(t, "vm", n, code, out.String(), errOut.String())
		if n == 1 && !refused {
			t.Errorf("the box site does not count as an allocation: exit %d stdout=%q", code, out.String())
		}
	}
}

// Native hands the nullary variant's immortal over without allocating, so the VM counts nothing.
const nullaryBoxSrc = `type Tick = Up(n: Int) | Flat

fn pick[T](v: T) -> T { v }

fn run(n: Int) -> Int {
    let t = if n > 0 { Up(n) } else { Flat }
    match pick(t) {
        Up(v) -> v
        Flat -> 0
    }
}

print "ok=${run(0)}"
`

func TestFaultInjectionNullaryBoxVM(t *testing.T) {
	t.Parallel()
	if code, out, errOut := vmAtFirstAlloc(t, nullaryBoxSrc); code != 0 || out != "ok=0\n" {
		t.Errorf("boxing a payload-free variant counted as an allocation: exit %d stdout=%q stderr=%q", code, out, errOut)
	}
}

func vmAtFirstAlloc(t *testing.T, src string) (int, string, string) {
	t.Helper()
	a, err := driver.Analyze(loadTryConstructorModule(t, src))
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatal(a.Res.Render())
	}
	var out, errOut bytes.Buffer
	code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: 1}, &out, &errOut)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	return code, out.String(), errOut.String()
}

const crossSrc = `fn pick%s { v }

fn run(n: Int) -> Int {
    if pick(Some(%s)) is Some(v) { v + n } else { n }
}

print "ok=${run(4)}"
`

func crossProgram(generic bool, payload string) string {
	sig := "(v: Int?) -> Int?"
	if generic {
		sig = "[T](v: T) -> T"
	}
	return fmt.Sprintf(crossSrc, sig, payload)
}

// A fixed crossing reads the variant's immortal and the VM counts nothing; one built from a parameter is a box it counts.
func TestFaultInjectionFixedBoxVM(t *testing.T) {
	t.Parallel()
	if code, out, errOut := vmAtFirstAlloc(t, crossProgram(true, "4")); code != 0 || out != "ok=8\n" {
		t.Errorf("handing over a fixed variant counted as an allocation: exit %d stdout=%q stderr=%q", code, out, errOut)
	}
	if code, _, errOut := vmAtFirstAlloc(t, crossProgram(true, "n")); code != 2 || !strings.Contains(errOut, "out of memory") {
		t.Errorf("boxing a variant built at run time was not refused: exit %d stderr=%q", code, errOut)
	}
}

func nativeAllocs(t *testing.T, src string) int {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(loadTryConstructorModule(t, src), exe, &buildErr, driver.BuildOptions{
		CFlags: []string{"-DKIGUMI_FAULT_INJECT"},
	})
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	for n := 1; n <= directBoxSweep*4; n++ {
		cmd := exec.Command(exe)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if !checkBoxOutcome(t, "native", n, exitCodeOf(cmd.Run()), out.String(), errOut.String()) {
			return n - 1
		}
	}
	t.Fatalf("native still refuses at N=%d", directBoxSweep*4)
	return 0
}

func TestFaultInjectionFixedBoxNative(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	if crossed, inFrame := nativeAllocs(t, crossProgram(true, "4")), nativeAllocs(t, crossProgram(false, "4")); crossed != inFrame {
		t.Errorf("handing over a fixed variant allocates: %d allocations, %d without the crossing", crossed, inFrame)
	}
	if crossed, inFrame := nativeAllocs(t, crossProgram(true, "n")), nativeAllocs(t, crossProgram(false, "n")); crossed <= inFrame {
		t.Errorf("boxing a variant built at run time allocates nothing: %d allocations, %d without the crossing", crossed, inFrame)
	}
}
