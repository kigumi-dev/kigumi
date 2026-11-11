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
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

const mapIntKeySweep = 200

// The program recoverable_map_int_key_ok accepts must turn every refused allocation into its Err, on both backends.
func TestRecoverableMapIntKeySurvivesSweep(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(fixturePath(t, "sem/recoverable_map_int_key_ok"))
	if err != nil {
		t.Fatal(err)
	}
	_, src, _ := strings.Cut(string(data), "-- src --\n")
	src, _, _ = strings.Cut(src, "\n-- ")
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"mapkey\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	target := buildconfig.HostTarget()
	target.Profile = "recoverable-only"
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Target: target})
	if err != nil {
		t.Fatal(err)
	}
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatal(a.Res.Render())
	}
	sweepToCompletion(t, "vm", func(n int) (int, string, string) {
		var out, errOut bytes.Buffer
		code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: int64(n)}, &out, &errOut)
		if err != nil {
			t.Fatalf("vm N=%d: %v", n, err)
		}
		return code, out.String(), errOut.String()
	})
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{CFlags: []string{"-DKIGUMI_FAULT_INJECT"}}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	sweepToCompletion(t, "native", func(n int) (int, string, string) {
		cmd := exec.Command(exe)
		cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		return exitCodeOf(cmd.Run()), out.String(), errOut.String()
	})
}

// sweepToCompletion refuses allocation n for n = 1, 2, ... until a run passes; only a refused or wrong run prints.
func sweepToCompletion(t *testing.T, backend string, run func(n int) (int, string, string)) {
	t.Helper()
	for n := 1; n <= mapIntKeySweep; n++ {
		code, out, errOut := run(n)
		switch {
		case code != 0:
			t.Fatalf("%s N=%d: exit %d, stdout=%q stderr=%q", backend, n, code, out, errOut)
		case out == "":
			if n == 1 {
				t.Fatalf("%s: the first allocation was not refused into an Err", backend)
			}
			return
		case out != "err\n":
			t.Fatalf("%s N=%d: stdout=%q", backend, n, out)
		}
	}
	t.Fatalf("%s: still refusing at N=%d", backend, mapIntKeySweep)
}
