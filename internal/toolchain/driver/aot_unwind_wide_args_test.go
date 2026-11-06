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

const wideSlotHarness = `#include "rt_core.c"
#include <stdio.h>

static int g_argc;

static val boom(val env, val args) {
    rt_panic("boom");
    return NULL;
}

static void* g_vt[1] = {(void*)boom};

static void run(val unused) {
    val args[40];
    for (int i = 0; i < g_argc; i++) args[i] = rt_int(1000 + i, 0);
    struct obj box = {.kind = K_BOX, .rc = -1};
    box.u.box.vt = g_vt;
    box.u.box.v = rt_int(7, 0);
    rt_call_slot(&box, 0, g_argc, args, 0);
}

int main(int argc, char** argv) {
    g_argc = atoi(argv[1]);
    int completed = contained(run, NULL);
    printf("completed=%d live=%lld\n", completed, (long long)g_general.live);
    return 0;
}
`

func buildWideHarness(t *testing.T, source string) func(n string) string {
	t.Helper()
	cc := testCompiler().CCompiler()
	if cc == nil {
		t.Skip("no C compiler")
	}
	rt, _ := filepath.Abs("../../backend/llgen/runtimegen/runtime")
	dir := t.TempDir()
	src := filepath.Join(dir, "harness.c")
	if err := os.WriteFile(src, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "harness")
	args := append(append([]string{}, cc[1:]...), "-I"+rt, "-DKIGUMI_TASK_CATCH", "-DKIGUMI_UNWIND", "-DKIGUMI_LEAK_CHECK", src, filepath.Join(rt, "rt_sys_weak.c"), filepath.Join(rt, "rt_float.c"), "-o", exe, "-lm")
	if out, err := exec.Command(cc[0], args...).CombinedOutput(); err != nil {
		t.Fatalf("build harness: %v\n%s", err, out)
	}
	return func(n string) string {
		out, err := exec.Command(exe, n).CombinedOutput()
		if err != nil {
			t.Fatalf("run with %s arguments: %v\n%s", n, err, out)
		}
		return strings.TrimSpace(string(out))
	}
}

func checkWideArgsMatchNarrow(t *testing.T, run func(string) string) {
	t.Helper()
	narrow := run("4")
	if !strings.HasPrefix(narrow, "completed=0 ") {
		t.Fatalf("the call must panic: %q", narrow)
	}
	for _, n := range []string{"17", "33"} {
		if got := run(n); got != narrow {
			t.Errorf("%s arguments: %q, want %q as with 4 arguments", n, got, narrow)
		}
	}
}

// A std function value called with more arguments than the stack buffer holds must give its argument region back when the primitive panics.
func TestWideStdCallPanicFreesItsArgumentRegionNative(t *testing.T) {
	checkWideArgsMatchNarrow(t, buildWideHarness(t, wideArgsHarness))
}

// A requirement slot called with more arguments than the stack buffer holds must give its argument region back when the callee panics.
func TestWideSlotCallPanicFreesItsArgumentRegionNative(t *testing.T) {
	checkWideArgsMatchNarrow(t, buildWideHarness(t, wideSlotHarness))
}
