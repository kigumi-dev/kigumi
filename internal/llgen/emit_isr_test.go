package llgen_test

import (
	"strings"
	"testing"
)

// TestExportInterruptWrapperNoAlloc checks the export(interrupt) wrapper and handler body never allocate.
func TestExportInterruptWrapperNoAlloc(t *testing.T) {
	src := `
noalloc fn poke() {}

export(interrupt) {
    pub fn tick() {
        poke()
    }
}
`
	ir := emitIR(t, src)
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

const isrNoTrapSrc = `type Mode = | Idle | Busy(Int)

fn pick(m: Mode) -> Int {
    match m {
        Idle -> 0
        Busy(n) -> n
    }
}

export(interrupt) {
    pub fn tick() {
        let b = true
        let r = match b {
            true -> 1
            false -> 2
        }
        let mut n = 0
        for i in 0..4 {
            n = n.wrappingAdd(i)
        }
        for i in 0..=r {
            n = n.wrappingAdd(i)
        }
        let mut m: u8 = 0
        let hi8: u8 = 3
        for j in m..hi8 {
            m = m.wrappingAdd(j)
        }
        let mut w: i128 = 0
        let hi128: i128 = 3
        for k in w..hi128 {
            w = w.wrappingAdd(k)
        }
        _ = n.wrappingAdd(pick(Idle))
    }
}
`

// TestExportInterruptNoTrapLeavesNoPanicPath checks the unreachable match fall-through and the guarded counter step compile to nothing.
func TestExportInterruptNoTrapLeavesNoPanicPath(t *testing.T) {
	ir := emitIR(t, isrNoTrapSrc)
	handler := bodyOf(t, ir, `define ptr @"main.tick$kg"(`)
	for name, body := range map[string]string{"handler": handler, "pick": fnBody(t, ir, "main.pick")} {
		for _, bad := range []string{"@rt_panic", "@rt_iadd", "with.overflow", "@llvm.trap", "@llvm.debugtrap"} {
			if strings.Contains(body, bad) {
				t.Errorf("%s reaches %s, a trap path the interrupt contract forbids:\n%s", name, bad, body)
			}
		}
	}
	for _, want := range []string{"add i64", "add i128"} {
		if !strings.Contains(handler, want) {
			t.Errorf("the guarded counter step must be a plain `%s`:\n%s", want, handler)
		}
	}
}
