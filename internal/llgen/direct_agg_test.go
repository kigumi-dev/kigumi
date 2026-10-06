package llgen_test

import (
	"strings"
	"testing"
)

const directAggSrc = `
type Point = {
    pub x Int
    pub y Int
}

type Box = {
    pub lo Point
    pub hi Point
    pub tight Bool
}

type Named = {
    pub at Point
    pub label String
}

fn mk(x: Int, y: Int) -> Point { Point { x: x, y: y } }

fn area(b: Box) -> Int { (b.hi.x - b.lo.x) * (b.hi.y - b.lo.y) }

fn grow(b: Box, k: Int) -> Box {
    Box { lo: mk(b.lo.x - k, b.lo.y - k), hi: mk(b.hi.x + k, b.hi.y + k), tight: false }
}

fn roundTrip(b: Box, k: Int) -> Int {
    let c = b
    area(grow(c, k)) + area(b)
}

fn swap(p: (Int, Int)) -> (Int, Int) {
    let (x, y) = p
    (y, x)
}

fn named(n: Named) -> Int { n.at.x }

fn main() -> Unit! {}
`

func TestDirectAggregateNeverAllocates(t *testing.T) {
	ir := emitIR(t, directAggSrc)
	for _, fn := range []string{`@"main.area"(`, `@"main.grow"(`, `@"main.roundTrip"(`, `@"main.swap"(`} {
		body := bodyOf(t, ir, fn)
		checkNoHeap(t, fn, body)
		for _, bad := range []string{"@rt_copy(", "@rt_retain(", `@"kg.box.`, `@"kg.unbox.`} {
			if strings.Contains(body, bad) {
				t.Errorf("%s crosses the boxed model with %s:\n%s", fn, bad, body)
			}
		}
	}
}

func TestCarrierFieldRecordStaysBoxed(t *testing.T) {
	ir := emitIR(t, directAggSrc)
	if strings.Contains(ir, `%"kg.agg.Named"`) {
		t.Error("a record with a String field was laid out as a direct aggregate")
	}
	if !strings.Contains(bodyOf(t, ir, `@"main.named"(`), "@rt_field(") {
		t.Error("the boxed record's field read no longer goes through rt_field")
	}
}

// Two machine words travel in registers; anything larger travels through
// caller storage, so the frame is the only place a big one lives.
func TestDirectAggregateCallingConvention(t *testing.T) {
	ir := emitIR(t, directAggSrc)
	if !strings.Contains(ir, `define %"kg.agg.Point" @"main.mk"(`) {
		t.Error("a 16-byte result is not returned in registers")
	}
	if !strings.Contains(ir, `define void @"main.grow"(ptr %sret, ptr %p0, ptr %p1)`) {
		t.Error("a result over the by-value threshold is not returned through caller storage")
	}
	if !strings.Contains(ir, `define %"kg.agg.(Int, Int)" @"main.swap"(%"kg.agg.(Int, Int)" %p0)`) {
		t.Error("a small tuple is not passed and returned in registers")
	}
	if !strings.Contains(ir, `%"kg.agg.Box" = type { %"kg.agg.Point", %"kg.agg.Point", i1 }`) {
		t.Error("the nested aggregate layout is not the explicit struct type")
	}
}

// A direct value reaching a carrier slot is one named call, not a silent fallback.
func TestDirectAggregateBoxesExplicitlyAtCarriers(t *testing.T) {
	ir := emitIR(t, `
type Point = {
    pub x Int
    pub y Int
}

fn stash(x: Int) -> Array[Point] {
    Array.of(Point { x: x, y: x })
}

fn main() -> Unit! {}
`)
	body := bodyOf(t, ir, `@"main.stash"(`)
	if !strings.Contains(body, `call ptr @"kg.box.Point"(`) {
		t.Errorf("storing a direct aggregate into an Array does not go through the boxing helper:\n%s", body)
	}
}

const lentReceiverSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn Point.combine(self, other: Point) -> Int { self.x + other.x }

fn touch(n: Int) -> Int {
    let p = Point { x: n, y: n + 1 }
    p.y + p.combine(Point { x: n + 2, y: n + 3 })
}

fn main() -> Unit! {
    print "${touch(1)}"
}
`

func callArg(t *testing.T, body, head string) string {
	t.Helper()
	i := strings.Index(body, head)
	if i < 0 {
		t.Fatalf("no %s in:\n%s", head, body)
	}
	rest := body[i+len(head):]
	return rest[:strings.IndexAny(rest, ",)")]
}

// A direct signature never covers a lent receiver: that box is the caller's to give back.
func TestLentDirectReceiverGivesItsBoxBack(t *testing.T) {
	body := bodyOf(t, emitIR(t, lentReceiverSrc), `@"main.touch"(`)
	if !strings.Contains(body, `call ptr @"kg.box.Point"(`) {
		t.Fatalf("the lent receiver does not cross through the boxing helper:\n%s", body)
	}
	recv := callArg(t, body, `call ptr @"main.Point.combine"(ptr `)
	if !strings.Contains(body, "call void @rt_release(ptr "+recv+")") {
		t.Errorf("the box handed to the lent receiver is never released:\n%s", body)
	}
}
