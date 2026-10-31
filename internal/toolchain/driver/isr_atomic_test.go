package driver_test

import (
	"bytes"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const isrAtomicSrc = `import {atomicAdd, atomicLoad, atomicCompareExchange, AtomicOrder} from std/mem

static mut ticks: u32 = 0
static mut armed: Bool = false
static mut total: u64 = 0

noalloc fn bump() {
    unsafe {
        _ = atomicAdd(&mut ticks, 1, AtomicOrder.SeqCst)
    }
}

export(interrupt) {
    pub fn tick() {
        unsafe {
            _ = atomicAdd(&mut ticks, 2, AtomicOrder.Relaxed)
            _ = atomicCompareExchange(&mut armed, false, true, AtomicOrder.AcqRel)
            _ = atomicLoad(&ticks, AtomicOrder.Acquire)
        }
        bump()
    }
}

fn main() -> Void! {
    tick()
    unsafe {
        _ = atomicAdd(&mut total, 1, AtomicOrder.SeqCst)
    }
}
`

const isrAtomicAllow = "Interrupt { allow: \"sync mem.atomic width=64 impl=native\" }\n"

func handlerIR(t *testing.T, ir, name string) string {
	t.Helper()
	start := strings.Index(ir, "define ptr @\""+name+"$kg\"(")
	if start < 0 {
		t.Fatalf("no handler body %s in IR", name)
	}
	end := strings.Index(ir[start:], "\n}\n")
	return ir[start : start+end]
}

func TestInterruptAtomicPermission(t *testing.T) {
	t.Parallel()
	res, err := driver.Check(isrModule(t, "", isrAtomicSrc))
	if err != nil {
		t.Fatal(err)
	}
	out := res.Render()
	for _, want := range []string{"E1088", "sync mem.atomic width=64 impl=native", "calls `atomicAdd`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("a handler without the permission must be refused (missing %q):\n%s", want, out)
		}
	}
	res, err = driver.Check(isrModule(t, isrAtomicAllow, isrAtomicSrc))
	if err != nil || res.HasErrors() {
		t.Fatalf("the permitted handler must pass the checker and the lowering check: %v\n%s", err, res.Render())
	}
}

func TestInterruptAtomicLowersInline(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(isrModule(t, isrAtomicAllow, isrAtomicSrc), &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
	body := handlerIR(t, ir, "isr.tick")
	for _, bad := range []string{"@calloc", "@free", "@rt_int(", "@rt_bool(", "@rt_release(", "@rt_std", "@rt_builtin"} {
		if strings.Contains(body, bad) {
			t.Errorf("the handler calls %s:\n%s", bad, body)
		}
	}
	for _, want := range []string{
		`define internal i64 @"kg.atomic.add.i64.32.monotonic"`, `cmpxchg ptr %p, i64 %old, i64 %new monotonic monotonic`,
		`define internal i64 @"kg.atomic.add.i64.64.seq_cst"`, `atomicrmw add ptr %p, i64 %v seq_cst`,
		`define internal i64 @"kg.atomic.add.i64.32.seq_cst"`, `cmpxchg ptr %p, i64 %old, i64 %new seq_cst seq_cst`,
		`cmpxchg ptr %p, i8 %x, i8 %v acq_rel acquire`, `load atomic i64, ptr %p acquire, align 8`,
	} {
		if !strings.Contains(ir, want) {
			t.Errorf("the IR lacks %q", want)
		}
	}
}

const isrMmioSrc = `import {volatileReadAt, volatileWriteAt} from std/mem

fn pulse() {
    unsafe {
        let status: u32 = volatileReadAt(0x40021000)
        volatileWriteAt(0x40021004, status | 1)
        let ready: Bool = volatileReadAt(0x40021008)
        volatileWriteAt(0x40021008, !ready)
        volatileWriteAt[u8](0x4002100c, 7)
    }
}

export(interrupt) {
    pub fn irq() {
        pulse()
    }
}

fn main() -> Void! {
    irq()
}
`

func TestInterruptMmioPermission(t *testing.T) {
	t.Parallel()
	res, err := driver.Check(isrModule(t, "", isrMmioSrc))
	if err != nil {
		t.Fatal(err)
	}
	out := res.Render()
	if !strings.Contains(out, "E1088") || !strings.Contains(out, "mmio mem.volatileReadAt") {
		t.Fatalf("a handler without the permission must be refused:\n%s", out)
	}
	res, err = driver.Check(isrModule(t, "Interrupt { allow: \"mmio mem.volatileReadAt\" }\n", isrMmioSrc))
	if err != nil || !strings.Contains(res.Render(), "mmio mem.volatileWriteAt") {
		t.Fatalf("each access is its own permission: %v\n%s", err, res.Render())
	}
	res, err = driver.Check(isrModule(t, "Interrupt { allow: \"mmio mem.volatileReadAt; mmio mem.volatileWriteAt\" }\n", isrMmioSrc))
	if err != nil || res.HasErrors() {
		t.Fatalf("the permitted handler must pass the checker and the lowering check: %v\n%s", err, res.Render())
	}
}

const volatileNativeSrc = `import ffi from std/ffi
import {volatileRead, volatileWrite, volatileReadAt, volatileWriteAt} from std/mem

fn probe(p: *mut u32, q: *mut u8) {
    unsafe {
        volatileWrite(p, volatileRead(ffi.toConst(p)) + 1)
        volatileWrite(q, 7)
    }
}

fn regs() {
    unsafe {
        let status: u16 = volatileReadAt(0x1000)
        volatileWriteAt(0x1002, status)
        let flag: Bool = volatileReadAt(0x1004)
        volatileWriteAt(0x1004, !flag)
        let wide: Int = volatileReadAt(0x1008)
        volatileWriteAt(0x1008, wide)
    }
}

fn main() -> Void! {
    regs()
    let seed32: u32 = 1
    let seed8: u8 = 2
    let word = ffi.CValue.new(seed32)
    let byte = ffi.CValue.new(seed8)
    probe(word.ptr(), byte.ptr())
}
`

func TestVolatileEmitsVolatileAccesses(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(isrModule(t, "", volatileNativeSrc), &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
	for _, want := range []string{
		"load volatile i32, ptr", "store volatile i32 ", "store volatile i8 ",
		"load volatile i16, ptr", "store volatile i16 ", "inttoptr i64 ", "load volatile i8, ptr", "load volatile i64, ptr", "store volatile i64 ",
	} {
		if !strings.Contains(ir, want) {
			t.Errorf("the IR lacks %q", want)
		}
	}
}

const volatileRegisterSrc = `import {volatileReadAt, volatileWriteAt} from std/mem

unsafe {
    let before: u32 = volatileReadAt(0x40021000)
    volatileWriteAt(0x40021000, before + 5)
    volatileWriteAt(0x40021004, true)
    let narrow: u8 = 250
    volatileWriteAt(0x40021008, narrow)
    let flag: Bool = volatileReadAt(0x40021004)
    let again: u32 = volatileReadAt(0x40021000)
    let b: u8 = volatileReadAt(0x40021008)
    let never: Int = volatileReadAt(0x40021ffc)
    print("${before} ${again} ${flag} ${b} ${never}")
}
`

func TestVolatileAtRegisterTableInVM(t *testing.T) {
	t.Parallel()
	m := isrModule(t, "", volatileRegisterSrc)
	var out, errOut bytes.Buffer
	if code, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, []string{"main"}); err != nil || code != 0 {
		t.Fatalf("vm: %v (exit %d)\n%s", err, code, errOut.String())
	}
	if got, want := strings.TrimSpace(out.String()), "0 5 true 250 0"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
