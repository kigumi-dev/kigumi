package plan_test

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/devkit/mirfixture"
)

const lendSrc = `
type Point = {
    pub x Int
    pub y Int
}

type Tick = Up(n: Int) | Flat

fn peek(p: &Point) -> Int { p.x * 10 + p.y }

fn forward(p: &Point) -> Int { peek(p) }

fn weigh(t: &Tick) -> Int {
    match t {
        Up(n) -> n
        Flat -> 0
    }
}

fn show(p: &Point) -> String { debug(p) }

fn fingerprint[T: Hash](a: &T) -> u64 { hash(a) }

fn hashes(i: Int) -> Bool { fingerprint(&Point { x: i, y: 1 }) == 7 }

fn lends(i: Int) -> Int {
    let p = Point { x: i, y: 1 }
    let t = Up(i)
    peek(&p) + forward(&p) + weigh(&t)
}

fn keeps(i: Int) -> Int {
    let p = Point { x: i, y: 1 }
    show(&p).len().toInt() || 0
}

fn main() -> Void! {}
`

func TestPlanDirectLendsAReadOnlyReferenceByValue(t *testing.T) {
	p := specialized(t, lendSrc)
	for _, name := range []string{"main.peek", "main.forward", "main.weigh"} {
		f := mirfixture.FuncNamed(t, p, name)
		if f.DSig.Param(0) == 0 {
			t.Errorf("%s still takes its reference as a carrier:\n%s", name, p.Dump())
		}
	}
	lends := mirfixture.FuncNamed(t, p, "main.lends")
	if n := mirfixture.CountOp(lends, midir.OpBorrow) + mirfixture.CountOp(lends, midir.OpBoxDirect); n != 0 {
		t.Errorf("lending locals still borrows or boxes %d times:\n%s", n, p.Dump())
	}
	hashes := mirfixture.FuncNamed(t, p, "main.hashes")
	if f := calleeOf(t, p, hashes, "fingerprint"); f.Name != "main.fingerprint[main.Point]" || f.DSig.Param(0) == 0 {
		t.Errorf("the generic helper does not take the record by value: %s %+v", f.Name, f.DSig)
	}
	if n := mirfixture.CountOp(hashes, midir.OpBorrow) + mirfixture.CountOp(hashes, midir.OpBoxDirect); n != 0 {
		t.Errorf("hashes still borrows or boxes %d times:\n%s", n, p.Dump())
	}
	if f := mirfixture.FuncNamed(t, p, "main.show"); f.DSig.Param(0) != 0 {
		t.Errorf("a reference that a std primitive reads must stay a carrier: %+v", f.DSig)
	}
	keeps := mirfixture.FuncNamed(t, p, "main.keeps")
	if mirfixture.CountOp(keeps, midir.OpBorrow) != 1 {
		t.Errorf("a borrow the callee needs as an object must stay:\n%s", p.Dump())
	}
	if err := mirverify.Program(p); err != nil {
		t.Fatalf("verify: %v", err)
	}
}
