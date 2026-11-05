package llgen_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/toolchain/version"
)

func TestRuntimeLayoutCoversTheSharedStructs(t *testing.T) {
	layout, err := llgen.RuntimeLayout()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"K_INT = 1", "K_PTR", "struct obj {", "struct rt_type {", "struct rt_vdesc {", "kg_unwind", "kg_uframe", "RT_OBJ_SLOT 64", "unwindHeaderWords 3", "%rt_type = type", "%rt_variant = type"} {
		if !strings.Contains(layout, want) {
			t.Errorf("layout text lacks %q", want)
		}
	}
	if strings.Contains(layout, "//") || strings.Contains(layout, "\n\n") {
		t.Error("layout text keeps comments or blank lines, so a comment edit would move the fingerprint")
	}
}

func TestAbiFingerprintInputs(t *testing.T) {
	layout, _ := llgen.RuntimeLayout()
	base := llgen.AbiFingerprint(opdef.Registry, layout, version.AbiRevision, []string{"KIGUMI_UNWIND", "KIGUMI_CAP_TASK_THREADS"})
	if got := llgen.AbiFingerprint(opdef.Registry, layout, version.AbiRevision, []string{"KIGUMI_CAP_TASK_THREADS", "KIGUMI_UNWIND", "KIGUMI_UNWIND"}); got != base {
		t.Error("flag order or repetition moved the fingerprint")
	}
	for name, got := range map[string]string{
		"a flag":   llgen.AbiFingerprint(opdef.Registry, layout, version.AbiRevision, []string{"KIGUMI_UNWIND"}),
		"a layout": llgen.AbiFingerprint(opdef.Registry, strings.Replace(layout, "K_PTR", "K_PTR2", 1), version.AbiRevision, []string{"KIGUMI_UNWIND", "KIGUMI_CAP_TASK_THREADS"}),
	} {
		if got == base {
			t.Errorf("changing %s left the fingerprint unchanged", name)
		}
	}
	if len(base) != len(llgen.AbiPlaceholder) {
		t.Errorf("fingerprint %q and placeholder %q differ in length", base, llgen.AbiPlaceholder)
	}
}

func TestEmitAbiCheckIsOptIn(t *testing.T) {
	prog := plannedProgram(t, "print \"x\"\n")
	if ir := llgen.EmitWith(prog, llgen.EmitOptions{}); strings.Contains(ir, "call void @rt_abi_check") {
		t.Error("an emit without AbiCheck calls rt_abi_check")
	}
	out := llgen.EmitProgram(prog, llgen.EmitOptions{AbiCheck: true})
	main := bodyOf(t, out.IR, "define i32 @main")
	first := strings.Split(strings.TrimSpace(main), "\n")[1]
	if !strings.Contains(first, "call void @rt_abi_check(ptr @.s") {
		t.Errorf("the entry's first instruction is %q, want the ABI check", first)
	}
	fp := llgen.CurrentAbiFingerprint(version.AbiRevision, nil)
	bound := out.BindAbi(fp)
	if strings.Contains(bound.IR, llgen.AbiPlaceholder) || !strings.Contains(bound.IR, fp) {
		t.Error("BindAbi did not install the fingerprint")
	}
	defer func() {
		if recover() == nil {
			t.Error("BindAbi accepted a fingerprint of the wrong length")
		}
	}()
	out.BindAbi("kgabi1:short")
}

// A new thread-creating call anywhere but rt_thread_pool_new would need its own capability, or the single-threaded runtime would race on plain refcounts.
// A blocking job's thread is the exception: it starts only under a thread-pool executor, whose capability already made the counts atomic.
func TestOnlyThreadPoolCreatesThreads(t *testing.T) {
	re := regexp.MustCompile(`pthread_create\(`)
	for _, f := range llgen.RuntimeFiles("posix", "linux") {
		for _, loc := range re.FindAllStringIndex(f.Src, -1) {
			before := f.Src[:loc[0]]
			if insideFn(before, "static blocking_job* blocking_job_start") && strings.Contains(f.Src, "ex->nworkers > 0) {\n        f->job = blocking_job_start(") {
				continue
			}
			if !insideFn(before, "val rt_thread_pool_new") {
				t.Errorf("%s creates a thread outside rt_thread_pool_new", f.Name)
			}
		}
	}
	src, err := os.ReadFile("runtime/rt_cap_task_executor.inc")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(re.FindAllIndex(src, -1)); n != 1 {
		t.Errorf("the executor creates threads at %d sites, want the one in rt_thread_pool_new", n)
	}
	op, ok := opdef.StdOpIdOf("TASK_THREAD_POOL_EXECUTOR")
	if c, has := op.Capability(); !ok || !has || c != opdef.CapTaskThreads {
		t.Errorf("the thread pool constructor's capability is %q, want task.threads", c)
	}
	for _, name := range opdef.StdOpNames {
		id, _ := opdef.StdOpIdOf(name)
		if c, has := id.Capability(); has && c == opdef.CapTaskThreads && name != "TASK_THREAD_POOL_EXECUTOR" {
			t.Errorf("op %s also needs task.threads", name)
		}
	}
}

func insideFn(before, header string) bool {
	fn := before[strings.LastIndex(before, "\n"+header)+1:]
	return strings.HasPrefix(fn, header) && !strings.Contains(fn[len(header):], "\n}\n")
}
