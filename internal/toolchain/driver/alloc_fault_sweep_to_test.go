package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/toolchain/driver"
)

// vmSweepExitsTo runs m on the VM with the allocation numbered 1..limit refused in turn; armed counts only after the first eprint.
func vmSweepExitsTo(t *testing.T, m *driver.Module, limit int, armed bool) []int {
	t.Helper()
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatal(a.Res.Render())
	}
	var exits []int
	for n := 1; n <= limit; n++ {
		var out, errOut bytes.Buffer
		code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: int64(n), FaultArmOnEprint: armed}, &out, &errOut)
		if err != nil {
			t.Fatalf("vm N=%d: %v", n, err)
		}
		exits = append(exits, code)
	}
	return exits
}

func nativeSweepExitsTo(t *testing.T, m *driver.Module, limit int, armed bool) []int {
	t.Helper()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{CFlags: []string{"-DKIGUMI_FAULT_INJECT"}}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	var exits []int
	for n := 1; n <= limit; n++ {
		cmd := exec.Command(exe)
		cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
		if armed {
			cmd.Env = append(cmd.Env, "KIGUMI_FAIL_ALLOC_ARM=1")
		}
		code, _, _ := runFaultCmd(cmd)
		exits = append(exits, code)
	}
	return exits
}
