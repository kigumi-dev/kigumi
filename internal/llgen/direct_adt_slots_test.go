package llgen_test

import (
	"strings"
	"testing"
)

const payloadSlotsProbe = `
type Point = {
    pub x Int
    pub y Int
}

type Mixed = A(a: u8, b: Int) | B(x: Float) | C(r: Point, flag: Bool) | D

fn twice(n: Int) -> Int { n * 2 }

fn describe(m: Mixed) -> Int {
    match m {
        A(a, b) -> twice(b) + a.toInt()
        B(x) -> if x > 1.0 { 7 } else { 8 }
        C(r, flag) -> r.x * 10 + r.y + (if flag { 100 } else { 0 })
        D -> 0 - 1
    }
}

fn main() -> Unit! {
    if describe(A(1, 2)) > 100 {}
}
`

func TestAdtPayloadSlotsAreSharedByPositionAndScalarsStayInRegisters(t *testing.T) {
	ir := specializedIR(t, payloadSlotsProbe)
	if !strings.Contains(ir, `%"kg.adt.main.Mixed" = type { i32, [3 x i64] }`) {
		t.Errorf("slot 0 must fit the widest first payload (Point, 16 bytes) and slot 1 the widest second (Int), after the tag:\n%s", ir)
	}
	body := bodyOf(t, ir, `define i64 @"main.describe"(`)
	checkNoHeap(t, "main.describe", body)
	for _, bad := range []string{"@rt_int(", "@rt_float(", `@"kg.box.`, "@rt_payload("} {
		if strings.Contains(body, bad) {
			t.Errorf("main.describe calls %s:\n%s", bad, body)
		}
	}
}
