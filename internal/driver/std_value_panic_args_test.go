package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/vm"
)

// The only print is a constant string, since an interpolation would allocate and could take the refused tick itself.
const stdValuePanicSrc = `import task from std/task
import alloc from std/alloc

type Res = resource {
    pub id Int
}

fn Res.drop(move self) -> Unit {
    print "drop"
}

fn make(f: fn(Res) -> alloc.Shared[Res], r: Res) -> alloc.Shared[Res] {
    f(r)
}

fn scan() -> Int {
    let r = Res { id: 1 }
    let build = alloc.Shared.new[Res]
    eprint "arm"
    let s = make(build, r)
    s.with((x: &Res) => x.id)
}

async fn boom() -> Int {
    scan()
}

async fn root() -> Int {
    let h = task.spawn(boom())
    await h
}

let ex = task.currentThreadExecutor()
print "total ${ex.runToCompletion(root())}"
`

func TestStdFunctionValuePanicGivesBackItsArgumentsVM(t *testing.T) {
	t.Parallel()
	a, err := driver.Analyze(loadTryConstructorModule(t, stdValuePanicSrc))
	if err != nil || a.Res.HasErrors() {
		t.Fatalf("analyze: %v", err)
	}
	var out, errOut bytes.Buffer
	code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: 1, FaultArmOnEprint: true}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if code != 2 || out.String() != "drop\n" || !strings.Contains(errOut.String(), "out of memory") {
		t.Errorf("exit %d, stdout %q, stderr %q; want the refused allocation to drop the argument once", code, out.String(), errOut.String())
	}
}

func TestStdFunctionValuePanicGivesBackItsArgumentsNative(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(loadTryConstructorModule(t, stdValuePanicSrc), exe, &buildErr, driver.BuildOptions{CFlags: []string{"-DKIGUMI_FAULT_INJECT"}})
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	cmd := exec.Command(exe)
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT=1", "KIGUMI_FAIL_ALLOC_ARM=1")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	code := exitCodeOf(cmd.Run())
	if code != 2 || out.String() != "drop\n" || !strings.Contains(errOut.String(), "out of memory") {
		t.Errorf("exit %d, stdout %q, stderr %q; want the refused allocation to drop the argument once", code, out.String(), errOut.String())
	}
}
