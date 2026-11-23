package llgen_test

import (
	"strings"
	"testing"
)

const staticScalarProbe = `
import alloc from std/alloc
export(C) { pub static mut saved: i64 = 0 }
noalloc fn get() -> i64 { unsafe { saved } }
fn main() -> Void! {
    unsafe { saved = 1024 }
    let arena = alloc.Arena.create()?
    allocator arena.scope() {
        let before = arena.allocated()
        let value = get()
        let after = arena.allocated()
        print "${value} ${after - before}"
    }
}
`

const volatileScalarProbe = `
import alloc from std/alloc
import ffi from std/ffi
import mem from std/mem
noalloc fn read(p: *const u64) -> u64 { unsafe { mem.volatileRead(p) } }
fn main() -> Void! {
    let cell = ffi.CValue.new[u64](1024)
    let pointer = ffi.toConst(cell.ptr())
    let arena = alloc.Arena.create()?
    allocator arena.scope() {
        let before = arena.allocated()
        let value = read(pointer)
        let after = arena.allocated()
        print "${value} ${after - before}"
    }
}
`

func TestStaticScalarReadStaysUnboxed(t *testing.T) {
	ir := specializedIR(t, staticScalarProbe)
	body := bodyOf(t, ir, "define i64 @\"main.get\"(")
	for _, bad := range []string{"@rt_int(", "@rt_box", "@rt_ptr("} {
		if strings.Contains(body, bad) {
			t.Errorf("static scalar read boxes through %s:\n%s", bad, body)
		}
	}
}

func TestVolatileScalarReadStaysUnboxed(t *testing.T) {
	ir := specializedIR(t, volatileScalarProbe)
	body := bodyOf(t, ir, "define i64 @\"main.read\"(")
	if !strings.Contains(body, "load volatile i64") {
		t.Fatalf("expected a native volatile load:\n%s", body)
	}
	for _, bad := range []string{"@rt_int(", "@rt_ptr_val", "@rt_ptr("} {
		if strings.Contains(body, bad) {
			t.Errorf("volatile scalar read boxes through %s:\n%s", bad, body)
		}
	}
}

func TestInlineAsmRecordStaysDirect(t *testing.T) {
	const src = `
type Regs = {
    pub eax u32
    pub ecx u32
}
noalloc fn read() -> u32 {
    let r: Regs = unsafe {
        asm {
            "nop"
            eax: out(eax)
            ecx: out(ecx)
        }
    }
    r.eax
}
fn main() -> Void! { print "${read()}" }
`
	ir := specializedIR(t, src)
	body := bodyOf(t, ir, "define i64 @\"main.read\"(")
	for _, bad := range []string{"@rt_record(", "@rt_int("} {
		if strings.Contains(body, bad) {
			t.Errorf("asm record result builds %s:\n%s", bad, body)
		}
	}
}
