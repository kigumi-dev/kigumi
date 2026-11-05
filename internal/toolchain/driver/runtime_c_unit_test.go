package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRuntimeCUnitContracts builds internal/backend/llgen/runtime/tests/rt_core_test.c and runs it:
// the C runtime has no unit tests of its own, so value ownership, copy, the drop hook, a contained
// panic and an allocator refusal are fixed here rather than only through the AOT corpus. The
// ambient KIGUMI_CFLAGS are added, so the CI sanitizer step exercises this harness under ASan.
func TestRuntimeCUnitContracts(t *testing.T) {
	cc := testCompiler().CCompiler()
	if cc == nil {
		t.Skip("no C compiler")
	}
	rt, _ := filepath.Abs("../../backend/llgen/runtime")
	dir := t.TempDir()
	exe := filepath.Join(dir, "rt_core_test")
	args := append(append([]string{}, cc[1:]...), "-I"+rt, "-g", "-O1",
		"-DKIGUMI_TASK_CATCH", "-DKIGUMI_UNWIND", "-DKIGUMI_LEAK_CHECK", "-DKIGUMI_FAULT_INJECT")
	args = append(args, strings.Fields(os.Getenv("KIGUMI_CFLAGS"))...)
	args = append(args, filepath.Join(rt, "tests", "rt_core_test.c"), filepath.Join(rt, "rt_sys_weak.c"), filepath.Join(rt, "rt_float.c"), "-lm", "-o", exe)
	if out, err := exec.Command(cc[0], args...).CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", strings.Join(cc, " "), err, out)
	}
	run := func(mode string, env []string) (string, string, error) {
		cmd := exec.Command(exe)
		if mode != "" {
			cmd.Args = append(cmd.Args, mode)
		}
		cmd.Env = append(os.Environ(), env...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		return stdout.String(), stderr.String(), err
	}
	if out, errOut, err := run("", nil); err != nil || out != "ok\n" {
		t.Fatalf("plain harness: %v\nstdout:\n%s\nstderr:\n%s", err, out, errOut)
	}
	if out, errOut, err := run("allocfail", []string{"KIGUMI_FAIL_ALLOC_AT=1"}); err != nil || out != "ok\n" {
		t.Fatalf("allocator-refusal harness: %v\nstdout:\n%s\nstderr:\n%s", err, out, errOut)
	}
}
