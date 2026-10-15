package llgen_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/llgen"
	"kigumi/internal/mir"
	"kigumi/internal/version"
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
	base := llgen.AbiFingerprint(mir.Registry, layout, version.AbiRevision, []string{"KIGUMI_UNWIND", "KIGUMI_CAP_TASK_THREADS"})
	if got := llgen.AbiFingerprint(mir.Registry, layout, version.AbiRevision, []string{"KIGUMI_CAP_TASK_THREADS", "KIGUMI_UNWIND", "KIGUMI_UNWIND"}); got != base {
		t.Error("flag order or repetition moved the fingerprint")
	}
	for name, got := range map[string]string{
		"a flag":   llgen.AbiFingerprint(mir.Registry, layout, version.AbiRevision, []string{"KIGUMI_UNWIND"}),
		"a layout": llgen.AbiFingerprint(mir.Registry, strings.Replace(layout, "K_PTR", "K_PTR2", 1), version.AbiRevision, []string{"KIGUMI_UNWIND", "KIGUMI_CAP_TASK_THREADS"}),
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
func TestOnlyThreadPoolCreatesThreads(t *testing.T) {
	re := regexp.MustCompile(`pthread_create\(`)
	for _, f := range llgen.RuntimeFiles("posix", "linux") {
		for _, loc := range re.FindAllStringIndex(f.Src, -1) {
			before := f.Src[:loc[0]]
			fn := before[strings.LastIndex(before, "\nval rt_thread_pool_new")+1:]
			if !strings.HasPrefix(fn, "val rt_thread_pool_new") || strings.Contains(fn[len("val rt_thread_pool_new"):], "\n}\n") {
				t.Errorf("%s creates a thread outside rt_thread_pool_new", f.Name)
			}
		}
	}
	src, err := os.ReadFile("runtime/rt_cap_task_executor.c.inc")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(re.FindAllIndex(src, -1)); n != 1 {
		t.Errorf("the executor creates threads at %d sites, want the one in rt_thread_pool_new", n)
	}
	op, ok := mir.StdOpIdOf("TASK_THREAD_POOL_EXECUTOR")
	if c, has := op.Capability(); !ok || !has || c != mir.CapTaskThreads {
		t.Errorf("the thread pool constructor's capability is %q, want task.threads", c)
	}
	for _, name := range mir.StdOpNames {
		id, _ := mir.StdOpIdOf(name)
		if c, has := id.Capability(); has && c == mir.CapTaskThreads && name != "TASK_THREAD_POOL_EXECUTOR" {
			t.Errorf("op %s also needs task.threads", name)
		}
	}
}
