package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/toolchain/driver"
)

const jsonBoxFaultSrc = `import json from std/json
import {Box} from std/alloc

type Holder = {
    pub n Box[Int]
}

match json.decode[Holder]("{\"n\": 5}") {
    Ok(h) -> print "ok ${h.n.get()}"
    Err(e) -> print "err ${e.message()}"
}
`

const jsonBoxFaultRange = 200

const jsonBoxOOM = "err out of memory while decoding"

func checkJsonBoxOutcome(t *testing.T, n, exit int, out, errOut string) bool {
	t.Helper()
	switch exit {
	case 0:
		if out != "ok 5\n" && !strings.HasPrefix(out, "err ") {
			t.Errorf("N=%d: unexpected output %q", n, out)
		}
		return strings.TrimSpace(out) == jsonBoxOOM
	case 2:
		if !strings.HasPrefix(errOut, "panic: ") {
			t.Errorf("N=%d: exit 2 without a panic message: %q", n, errOut)
		}
	default:
		t.Errorf("N=%d: exit %d, stdout=%q stderr=%q", n, exit, out, errOut)
	}
	return false
}

func TestFaultInjectionJSONBoxDecodeVM(t *testing.T) {
	t.Parallel()
	a, err := driver.Analyze(loadTryConstructorModule(t, jsonBoxFaultSrc))
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatal(a.Res.Render())
	}
	reached := false
	for n := 1; n <= jsonBoxFaultRange; n++ {
		var out, errOut bytes.Buffer
		code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: int64(n)}, &out, &errOut)
		if err != nil {
			t.Fatalf("N=%d: run error: %v", n, err)
		}
		reached = checkJsonBoxOutcome(t, n, code, out.String(), errOut.String()) || reached
	}
	if !reached {
		t.Error("no N reached the Box allocation error")
	}
}

func TestFaultInjectionJSONBoxDecodeNative(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(loadTryConstructorModule(t, jsonBoxFaultSrc), exe, &buildErr, driver.BuildOptions{
		CFlags: []string{"-DKIGUMI_FAULT_INJECT", "-DKIGUMI_RC_CHECK"},
	})
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	reached := false
	for n := 1; n <= jsonBoxFaultRange; n++ {
		cmd := exec.Command(exe)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		reached = checkJsonBoxOutcome(t, n, exitCodeOf(cmd.Run()), out.String(), errOut.String()) || reached
	}
	if !reached {
		t.Error("no N reached the Box allocation error")
	}
}
