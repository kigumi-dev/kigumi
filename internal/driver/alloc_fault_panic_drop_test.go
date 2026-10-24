package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/vm"
)

func panicDropSource(ctor string) string {
	return "import alloc from std/alloc\n\ntype Res = resource {\n    pub id Int\n}\n\nfn Res.drop(move self) {\n    print \"drop ${self.id}\"\n}\n\n" +
		"fn run() {\n    let r = Res{id: 1}\n    eprint \"go\"\n    let _ = " + ctor + "(r)\n}\n\nrun()\n"
}

// TestFaultInjectionPanickingNewDropsTheMovedValue pins that a refused Box.new / Shared.new drops the value moved in before it panics, on both backends.
func TestFaultInjectionPanickingNewDropsTheMovedValue(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range []struct {
		ctor        string
		nativeTicks int
	}{{"alloc.Box.new", 2}, {"alloc.Shared.new", 3}} {
		t.Run(c.ctor, func(t *testing.T) {
			t.Parallel()
			src := panicDropSource(c.ctor)
			a, err := driver.Analyze(loadTryConstructorModule(t, src))
			if err != nil || a.Res.HasErrors() {
				t.Fatalf("analyze: %v\n%s", err, a.Res.Render())
			}
			var out, errOut bytes.Buffer
			code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: 1, FaultArmOnEprint: true}, &out, &errOut)
			if err != nil {
				t.Fatalf("vm: %v", err)
			}
			checkPanicDrop(t, "vm", 1, code, out.String(), errOut.String())

			exe := buildFaultInjectable(t, src)
			for n := 1; n <= c.nativeTicks; n++ {
				cmd := exec.Command(exe)
				cmd.Dir = t.TempDir()
				cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_ARM=1", "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
				var nout, nerr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &nout, &nerr
				checkPanicDrop(t, "native", n, exitCodeOf(cmd.Run()), nout.String(), nerr.String())
			}
		})
	}
}

func checkPanicDrop(t *testing.T, backend string, n, exit int, stdout, stderr string) {
	t.Helper()
	if exit != 2 || !strings.HasSuffix(stderr, "panic: out of memory\n") || stdout != "drop 1\n" {
		t.Errorf("%s N=%d: exit %d stdout %q stderr %q, want the out-of-memory panic after one drop", backend, n, exit, stdout, stderr)
	}
}
