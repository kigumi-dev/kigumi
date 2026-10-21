package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/vm"
)

// A print allocates on native and not on the VM, so the only eprint is the one that arms the injection.
var tickSweepCases = []struct{ name, src string }{
	{"closure returned from a function", `fn mk(k: Int) -> fn(Int) -> Int { (x: Int) => x + k }
fn run(n: Int) -> Int {
    eprint "start"
    let f = mk(5)
    let mut s = 0
    for i in 0..n { s = s + f(i + 3000) }
    s
}
run(3)`},
	{"float fn item with a constant argument", `fn half(x: Float) -> Float { x / 2.0 }
fn ap(f: fn(Float) -> Float, x: Float) -> Float { f(x) }
fn run(n: Int) -> Float {
    eprint "start"
    let mut s = 0.0
    for i in 0..n { s = s + ap(half, 1.5) }
    s
}
run(3)`},
	{"parameter stored into an array element", `fn put(xs: &mut Array[Int], i: Int) { xs[0] = i }
fn run(n: Int) -> Int {
    let mut xs = Array.of(0, 0)
    eprint "start"
    for i in 0..n { put(&mut xs, i + 2000) }
    xs[0]
}
run(3)`},
	{"Option.map over a computed int", `fn run(n: Int) -> Int {
    eprint "start"
    let mut s = 0
    for i in 0..n {
        let p = Some(i + 5000).map((x: Int) => x * 2)
        if p is Some(_) { s = s + 1 }
    }
    s
}
run(3)`},
	{"parameter stored, argument is a user call result", `fn put(r: &mut Int, i: Int) { r = i }
fn f(x: Int) -> Int { x * 3 }
fn run(n: Int) -> Int {
    let mut c = 0
    eprint "start"
    for i in 0..n { put(&mut c, f(i + 1000)) }
    c
}
run(3)`},
	{"parameter stored, argument is a mutable accumulator", `fn put(r: &mut Int, i: Int) { r = i }
fn run(n: Int) -> Int {
    let mut c = 0
    let mut k = 2000
    eprint "start"
    for i in 0..n { k = k + 1
        put(&mut c, k) }
    c
}
run(3)`},
}

type refusedRun struct {
	exit int
	oom  bool
}

func vmRefusedRuns(t *testing.T, src string) []refusedRun {
	t.Helper()
	a, err := driver.Analyze(loadTryConstructorModule(t, src))
	if err != nil || a.Res.HasErrors() {
		t.Fatalf("analyze: %v", err)
	}
	var runs []refusedRun
	for n := 1; n <= directBoxSweep; n++ {
		var out, errOut bytes.Buffer
		code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: int64(n), FaultArmOnEprint: true}, &out, &errOut)
		if err != nil {
			t.Fatalf("vm N=%d: %v", n, err)
		}
		runs = append(runs, refusedRun{code, strings.Contains(errOut.String(), "out of memory")})
		if code == 0 {
			return runs
		}
	}
	t.Fatalf("the VM still refuses at N=%d", directBoxSweep)
	return nil
}

func nativeRefusedRuns(t *testing.T, src string, count int) []refusedRun {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(loadTryConstructorModule(t, src), exe, &buildErr, driver.BuildOptions{CFlags: []string{"-DKIGUMI_FAULT_INJECT"}})
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	var runs []refusedRun
	for n := 1; n <= count; n++ {
		cmd := exec.Command(exe)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n), "KIGUMI_FAIL_ALLOC_ARM=1")
		var errOut bytes.Buffer
		cmd.Stderr = &errOut
		code := exitCodeOf(cmd.Run())
		runs = append(runs, refusedRun{code, strings.Contains(errOut.String(), "out of memory")})
	}
	return runs
}

func TestFaultInjectionRefusedTickEndsBothBackendsTheSameWay(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range tickSweepCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			vmRuns := vmRefusedRuns(t, c.src)
			if len(vmRuns) < 4 {
				t.Fatalf("the program allocates only %d times", len(vmRuns)-1)
			}
			nativeRuns := nativeRefusedRuns(t, c.src, len(vmRuns))
			for i := range vmRuns {
				if vmRuns[i] != nativeRuns[i] {
					t.Errorf("N=%d: the VM ends with exit %d (out of memory: %v), native with exit %d (out of memory: %v)", i+1, vmRuns[i].exit, vmRuns[i].oom, nativeRuns[i].exit, nativeRuns[i].oom)
				}
			}
		})
	}
}
