package mir_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
)

func dprims(f *mir.Func) map[opdef.DirectPrim]int {
	out := map[opdef.DirectPrim]int{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == mir.OpCall && in.DPrim != opdef.DPNone {
				out[in.DPrim]++
			}
		}
	}
	return out
}

const directPrimSrc = `
import math from std/math

fn narrow(n: Int) -> u16? {
    n.toU16()
}

fn widen(n: u8) -> Int {
    n.toInt()
}

fn byteAt(data: Bytes, i: usize) -> u8 {
    data.get(i) || 0
}

fn pick(xs: &Array[Int], i: usize) -> Int? {
    xs.get(i)
}

fn viaRef(xs: &Array[String], i: usize) -> String? {
    xs.get(i)
}

fn spin(a: u32, b: u32) -> u32 {
    a.wrappingAdd(b).rotateLeft(b)
}

fn hyp(x: Float, y: Float) -> Float {
    math.sqrt(x * x + y * y)
}

fn main() -> Void! {}
`

func TestPlanDirectTakesADirectResultFromAPrimitive(t *testing.T) {
	p := planned(t, directPrimSrc)
	cases := []struct {
		fn   string
		kind opdef.DirectPrim
	}{
		{"main.narrow", opdef.DPConvert},
		{"main.widen", opdef.DPConvert},
		{"main.byteAt", opdef.DPBytesGet},
		{"main.pick", opdef.DPArrayGet},
		{"main.spin", opdef.DPWrapAdd},
		{"main.spin", opdef.DPRotl},
		{"main.hyp", opdef.DPSqrt},
	}
	for _, c := range cases {
		if got := dprims(funcNamed(t, p, c.fn))[c.kind]; got != 1 {
			t.Errorf("%s: %d direct-result calls of kind %d, want 1", c.fn, got, c.kind)
		}
	}
	if n := countOp(funcNamed(t, p, "main.narrow"), mir.OpUnboxDirect); n != 0 {
		t.Errorf("main.narrow unboxes %d times; the primitive's Option arrives direct", n)
	}
}

func TestPlanDirectKeepsAnElementThatIsNotALeafBoxed(t *testing.T) {
	f := funcNamed(t, planned(t, directPrimSrc), "main.viaRef")
	if got := dprims(f); len(got) != 0 {
		t.Errorf("an Option of a String is no leaf, yet calls took a direct result: %v", got)
	}
}

func TestStdOpResultABI(t *testing.T) {
	for _, name := range []string{"BYTES_GET", "ARRAY_GET", "CHAR_FROM_INT", "TEXT_PARSE_FLOAT", "LEN", "ARRAY_LEN"} {
		id, ok := opdef.StdOpIdOf(name)
		if !ok || id.ResultABI() != opdef.ResultDirect {
			t.Errorf("%s: result ABI is not direct", name)
		}
	}
	for _, name := range []string{"ARRAY_PUSH", "STRING_FROM_BYTES", "HOST_FILES", "ARRAY_TAKE_AT"} {
		id, ok := opdef.StdOpIdOf(name)
		if !ok || id.ResultABI() != opdef.ResultBoxed {
			t.Errorf("%s: result ABI is not boxed", name)
		}
	}
}

const capturedPrimSrc = `
fn passg[T](o: T) -> T {
    o
}

fn viaCell(n: Int) -> u8 {
    let mut x: u8? = None
    let g = () => {
        x = n.toU8()
    }
    g()
    x || 3
}

fn viaGeneric(n: Int) -> u16 {
    passg(n.toU16()) || 9
}

fn viaMatch(n: Int) -> u8 {
    match n.toU8() {
        Some(v) -> v
        None -> 0
    }
}

fn main() -> Void! {}
`

func TestPlanDirectKeepsABoxedResultWhereTheOptionWouldBeBoxedAgain(t *testing.T) {
	p := planned(t, capturedPrimSrc)
	for _, name := range []string{"main.viaCell.closure1", "main.viaGeneric"} {
		f := funcNamed(t, p, name)
		if got := dprims(f); len(got) != 0 {
			t.Errorf("%s: a result that is boxed again took a direct result: %v", name, got)
		}
		if n := countOp(f, mir.OpBoxDirect); n != 0 {
			t.Errorf("%s: %d box crossings, want none", name, n)
		}
	}
	if got := dprims(funcNamed(t, p, "main.viaMatch"))[opdef.DPConvert]; got != 1 {
		t.Errorf("main.viaMatch: %d direct-result conversions, want 1", got)
	}
}
