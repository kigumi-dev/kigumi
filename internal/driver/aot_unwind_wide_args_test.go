package driver_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const wideArgsHarness = `#include "rt_core.c"
#include <stdio.h>

static val g_std_fn;
static int g_argc;

static void run(val unused) {
    val args[40];
    for (int i = 0; i < g_argc; i++) args[i] = rt_int(i, 0);
    rt_callv(g_std_fn, g_argc, args);
}

int main(int argc, char** argv) {
    g_argc = atoi(argv[1]);
    g_std_fn = rt_std_closure("zz.no.such.key", 0, NULL);
    int completed = contained(run, NULL);
    rt_release(g_std_fn);
    printf("completed=%d live=%lld\n", completed, (long long)g_general.live);
    return 0;
}
`

// A std function value called with more arguments than the stack buffer holds must give its argument region back when the primitive panics.
func TestWideStdCallPanicFreesItsArgumentRegionNative(t *testing.T) {
	cc := testCompiler().CCompiler()
	if cc == nil {
		t.Skip("no C compiler")
	}
	rt, _ := filepath.Abs("../llgen/runtime")
	dir := t.TempDir()
	src := filepath.Join(dir, "harness.c")
	if err := os.WriteFile(src, []byte(wideArgsHarness), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "harness")
	args := append(append([]string{}, cc[1:]...), "-I"+rt, "-DKIGUMI_TASK_CATCH", "-DKIGUMI_UNWIND", "-DKIGUMI_LEAK_CHECK", src, filepath.Join(rt, "rt_sys.c"), filepath.Join(rt, "rt_float.c"), "-o", exe)
	if out, err := exec.Command(cc[0], args...).CombinedOutput(); err != nil {
		t.Fatalf("build harness: %v\n%s", err, out)
	}
	run := func(n string) string {
		out, err := exec.Command(exe, n).CombinedOutput()
		if err != nil {
			t.Fatalf("run with %s arguments: %v\n%s", n, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	narrow := run("4")
	if !strings.HasPrefix(narrow, "completed=0 ") {
		t.Fatalf("the unknown primitive must panic: %q", narrow)
	}
	for _, n := range []string{"17", "33"} {
		if got := run(n); got != narrow {
			t.Errorf("%s arguments: %q, want %q as with 4 arguments", n, got, narrow)
		}
	}
}
