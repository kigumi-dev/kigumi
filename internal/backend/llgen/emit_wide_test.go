package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
	mirreach "kigumi/internal/compiler/mir/reach"
)

const wideLoop = `
fn mac(n: u32) -> u128 {
    let mut acc: u128 = 1
    let mut i: u32 = 0
    for i < n {
        acc = acc * 3 + 7
        acc = acc.wrappingMul(5).wrappingAdd(acc >> 3)
        i = i + 1
    }
    acc
}

fn main() -> Void! {
    print "${mac(10)}"
}
`

func TestWideIntegersStayInRegisters(t *testing.T) {
	ir := emitIR(t, wideLoop)
	body := fnBody(t, ir, "main.mac")
	for _, want := range []string{"@llvm.uadd.with.overflow.i128", "@kg.umulo128(", "mul i128", "add i128", "lshr i128"} {
		if !strings.Contains(body, want) {
			t.Errorf("main.mac lacks %s:\n%s", want, body)
		}
	}
	for _, bad := range []string{"@rt_binop(", "@rt_std_id(", "@rt_box_int128("} {
		if strings.Contains(body, bad) {
			t.Errorf("main.mac goes through %s although every value is a register:\n%s", bad, body)
		}
	}
	for _, bad := range []string{"udiv i128", "sdiv i128", "urem i128", "srem i128", "@llvm.smul.with.overflow.i128", "@llvm.umul.with.overflow.i128"} {
		if strings.Contains(ir, bad) {
			t.Errorf("the module uses %s, which needs compiler-rt's __divti3 family or __muloti4", bad)
		}
	}
}

func TestInt128RuntimeIsAskedForOnlyWhenUsed(t *testing.T) {
	prog := plannedProgram(t, "fn main() -> Void! {\n    let x: u64 = 5\n    print \"${x * 2}\"\n}\n")
	plain := llgen.EmitProgram(prog, llgen.EmitOptions{Reachable: mirreach.Reachable(prog.R, prog.Entry, prog.Funcs, prog.ByEnt)})
	if plain.Int128 {
		t.Error("a program without 128-bit integers asks for the 128-bit runtime")
	}
	wp := plannedProgram(t, wideLoop)
	wide := llgen.EmitProgram(wp, llgen.EmitOptions{Reachable: mirreach.Reachable(wp.R, wp.Entry, wp.Funcs, wp.ByEnt)})
	if !wide.Int128 {
		t.Error("a program with a u128 does not ask for the 128-bit runtime")
	}
	if strings.Contains(plain.IR, "llvm.uadd.with.overflow.i128") {
		t.Error("the 128-bit intrinsics are declared in a program without 128-bit integers")
	}
}

func TestWideLiteralsReachTheRuntimeAsWords(t *testing.T) {
	ir := emitIR(t, `
fn main() -> Void! {
    let big: u128 = 340282366920938463463374607431768211455
    let neg: i128 = -170141183460469231731687303715884105728
    print "${big} ${neg}"
}
`)
	init := bodyOf(t, ir, "define void @kg_init_immortals() {")
	if !strings.Contains(init, "@rt_const_int128(") {
		t.Errorf("kg_init_immortals does not build the 128-bit literals:\n%s", init)
	}
	if !strings.Contains(init, "i64 -1, i64 -1") || !strings.Contains(init, "i64 0, i64 -9223372036854775808") {
		t.Errorf("the literal words are not the two's complement halves:\n%s", init)
	}
}
