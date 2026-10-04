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

// faultSweepSrc exercises every fallible growth path without ever
// unwrapping a Result, so a failure anywhere surfaces as "fail:*", never a crash.
const faultSweepSrc = `import map from std/map
import {Shared} from std/alloc

fn growArray() -> String {
    let mut a = Array.empty[Int]()
    let mut i: Int = 0
    for i < 40 {
        let before = a.len()
        if a.tryPush(i) is Err(_) {
            return if a.len() == before { "fail:${before}" } else { "BUG" }
        }
        i = i + 1
    }
    "ok"
}

fn growMap() -> String {
    let mut m = map.empty[Int, Int]()
    let mut i: Int = 0
    for i < 40 {
        let before = m.len()
        if m.trySet(i, i) is Err(_) {
            return if m.len() == before { "fail:${before}" } else { "BUG" }
        }
        i = i + 1
    }
    "ok"
}

fn bytesOps() -> String {
    if Bytes.tryZeros(8) is Err(_) {
        return "fail:zeros"
    }
    if Bytes.tryFill(1, 8) is Err(_) {
        return "fail:fill"
    }
    if Bytes.tryFromArray(&Array.of(1, 2, 3)) is Err(_) {
        return "fail:fromArray"
    }
    match Bytes.tryZeros(4) {
        Ok(za) -> match Bytes.tryZeros(4) {
            Ok(zb) -> if za.tryConcat(zb) is Err(_) { "fail:concat" } else { "ok" }
            Err(_) -> "fail:zeros2"
        }
        Err(_) -> "fail:zeros1"
    }
}

fn sharedOp() -> String {
    if Shared.tryNew(1) is Err(_) {
        return "fail:shared"
    }
    "ok"
}

print "${bytesOps()} ${sharedOp()} ${growArray()} ${growMap()}"
`

// sweepRange covers both backends' tick budget: the one-shot Bytes/Shared
// ops, then the two 40-iteration growth loops out to several hundred ticks.
const sweepRange = 450

func loadFaultSweepModule(t *testing.T) *driver.Module {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"faultsweep\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(faultSweepSrc), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func checkFaultOutcome(t *testing.T, n int, exit int, out, errOut string) {
	t.Helper()
	switch exit {
	case 0:
		if strings.Contains(out, "BUG") {
			t.Errorf("N=%d: strong failure guarantee violated: %q", n, out)
		}
	case 2:
		if !strings.HasPrefix(errOut, "panic: ") {
			t.Errorf("N=%d: exit 2 without a panic message: %q", n, errOut)
		}
	default:
		t.Errorf("N=%d: exit %d (want 0 or 2), stdout=%q stderr=%q", n, exit, out, errOut)
	}
}

func TestFaultInjectionSweepNative(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	m := loadFaultSweepModule(t)
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{
		CFlags: []string{"-DKIGUMI_FAULT_INJECT", "-DKIGUMI_RC_CHECK"},
	})
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	sawFail := map[string]bool{}
	for n := 1; n <= sweepRange; n++ {
		cmd := exec.Command(exe)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		err := cmd.Run()
		exit := exitCodeOf(err)
		checkFaultOutcome(t, n, exit, out.String(), errOut.String())
		if exit == 0 {
			recordFails(sawFail, out.String())
		}
	}
	requireAllFailKinds(t, "native", sawFail)
}

func recordFails(sawFail map[string]bool, out string) {
	for _, tok := range strings.Fields(out) {
		if strings.HasPrefix(tok, "fail:") {
			sawFail[tok] = true
			sawFail["fail:any"] = true
		}
	}
}

func requireAllFailKinds(t *testing.T, backend string, sawFail map[string]bool) {
	t.Helper()
	for _, want := range []string{"fail:any", "fail:zeros", "fail:fill", "fail:fromArray", "fail:shared"} {
		if !sawFail[want] {
			t.Errorf("%s: no N in 1..%d reached %s", backend, sweepRange, want)
		}
	}
}

func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}

// TestFaultInjectionSweepVM sweeps Options.FaultInjectAt over the same
// program on the VM backend, rerunning the analyzed program directly to skip driver.RunWith's checker pass.
func TestFaultInjectionSweepVM(t *testing.T) {
	m := loadFaultSweepModule(t)
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatal(a.Res.Render())
	}
	sawFail := map[string]bool{}
	for n := 1; n <= sweepRange; n++ {
		var out, errOut bytes.Buffer
		code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: int64(n)}, &out, &errOut)
		if err != nil {
			t.Fatalf("N=%d: run error: %v", n, err)
		}
		checkFaultOutcome(t, n, code, out.String(), errOut.String())
		if code == 0 {
			recordFails(sawFail, out.String())
		}
	}
	requireAllFailKinds(t, "vm", sawFail)
}

// tryConstructorCase isolates one try-constructor's allocation window from
// fatal setup (int/array literals); start is an empirical tick past that.
type tryConstructorCase struct {
	name  string
	src   string
	start int
}

// tryConstructorCases discards its Result so a leftover fatal call inside
// the constructor shows as a stray panic, not a branch.
var tryConstructorCases = []tryConstructorCase{
	{"Bytes.tryZeros", "fn run() -> Unit { _ = Bytes.tryZeros(8) }\nrun()\n", 2},
	{"Bytes.tryFill", "fn run() -> Unit { _ = Bytes.tryFill(1, 8) }\nrun()\n", 3},
	{"Bytes.tryConcat", "fn run() -> Unit {\n    match Bytes.tryZeros(4) {\n        Ok(z) -> { _ = z.tryConcat(z) }\n        Err(_) -> {}\n    }\n}\nrun()\n", 2},
	{"Bytes.tryFromArray", "fn run() -> Unit {\n    let a: Array[u8] = Array.of(1, 2, 3)\n    _ = Bytes.tryFromArray(&a)\n}\nrun()\n", 6},
	{"String.tryFromBytes", "fn run() -> Unit {\n    match Bytes.tryZeros(4) {\n        Ok(z) -> { _ = String.tryFromBytes(z) }\n        Err(_) -> {}\n    }\n}\nrun()\n", 2},
	{"Shared.tryNew", "import {Shared} from std/alloc\n\nfn run() -> Unit {\n    _ = Shared.tryNew(41)\n}\nrun()\n", 2},
}

// tryConstructorSweepWidth covers each case's own allocation points (a
// handful of ticks) many times over past its start.
const tryConstructorSweepWidth = 60

func loadTryConstructorModule(t *testing.T, src string) *driver.Module {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"tryctor\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestFaultInjectionTryConstructorsNative(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range tryConstructorCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := loadTryConstructorModule(t, c.src)
			exe := filepath.Join(t.TempDir(), "prog")
			var buildErr bytes.Buffer
			ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{
				CFlags: []string{"-DKIGUMI_FAULT_INJECT", "-DKIGUMI_RC_CHECK"},
			})
			if err != nil || !ok {
				t.Fatalf("build: %v\n%s", err, buildErr.String())
			}
			for n := c.start; n < c.start+tryConstructorSweepWidth; n++ {
				cmd := exec.Command(exe)
				cmd.Dir = t.TempDir()
				cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
				var out, errOut bytes.Buffer
				cmd.Stdout, cmd.Stderr = &out, &errOut
				err := cmd.Run()
				if exit := exitCodeOf(err); exit != 0 {
					t.Errorf("N=%d: exit %d, want 0 past setup: stdout=%q stderr=%q", n, exit, out.String(), errOut.String())
				}
			}
		})
	}
}

func TestFaultInjectionTryConstructorsVM(t *testing.T) {
	t.Parallel()
	for _, c := range tryConstructorCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := loadTryConstructorModule(t, c.src)
			a, err := driver.Analyze(m)
			if err != nil {
				t.Fatal(err)
			}
			if a.Res.HasErrors() {
				t.Fatal(a.Res.Render())
			}
			for n := c.start; n < c.start+tryConstructorSweepWidth; n++ {
				var out, errOut bytes.Buffer
				code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: int64(n)}, &out, &errOut)
				if err != nil {
					t.Fatalf("N=%d: run error: %v", n, err)
				}
				if code != 0 {
					t.Errorf("N=%d: exit %d, want 0 past setup: stdout=%q stderr=%q", n, code, out.String(), errOut.String())
				}
			}
		})
	}
}
