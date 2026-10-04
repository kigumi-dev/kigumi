package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/llgen"
	"kigumi/internal/mir"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

// TestExportInterruptWrapperNoAlloc checks the export(interrupt) wrapper and handler body never allocate.
func TestExportInterruptWrapperNoAlloc(t *testing.T) {
	src := `
noalloc fn poke() -> Unit {}

export(interrupt) {
    pub fn tick() -> Unit {
        poke()
    }
}
`
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.Check(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)})
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}); err != nil {
		t.Fatal(err)
	}
	ir := llgen.Emit(prog)
	start := strings.Index(ir, "define void @tick()")
	if start < 0 {
		t.Fatalf("no export(interrupt) wrapper for `tick` in IR:\n%s", ir)
	}
	end := strings.Index(ir[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("wrapper body not closed:\n%s", ir)
	}
	body := ir[start : start+end]
	checkNoAlloc(t, "wrapper", body)

	hstart := strings.Index(ir, `define ptr @"main.tick$kg"(`)
	if hstart < 0 {
		t.Fatalf("no handler body for `tick` in IR:\n%s", ir)
	}
	hend := strings.Index(ir[hstart:], "\n}\n")
	if hend < 0 {
		t.Fatalf("handler body not closed:\n%s", ir)
	}
	checkNoAlloc(t, "handler body", ir[hstart:hstart+hend])
}

func checkNoAlloc(t *testing.T, what, body string) {
	t.Helper()
	for _, bad := range []string{"@calloc", "@free", "@rt_int(", "@rt_float(", "@rt_ptr(", "@rt_char(", "@rt_release(", "@rt_record(", "@rt_variant(", "@rt_array(", "@rt_closure(", "@rt_closure_async("} {
		if strings.Contains(body, bad) {
			t.Errorf("export(interrupt) %s calls %s, which can allocate or free:\n%s", what, bad, body)
		}
	}
}
