package llgen_test

import (
	"strings"
	"testing"
)

const mutRefSrc = `
type Shape = Circle(r: Int) | Dot

fn Shape.area(self) -> Int {
    match self {
        Circle(r) -> r
        Dot -> 0
    }
}

fn reset(s: &mut Shape, k: Int) -> Unit {
    s = Circle(k)
}

fn forward(s: &mut Shape, k: Int) -> Unit {
    reset(s, k)
}

fn readOnly(s: &mut Shape) -> Int {
    s.area()
}

fn replaceLocal(k: Int) -> Int {
    let mut v: Shape = Dot
    reset(&mut v, k)
    v.area()
}

fn readLocal(k: Int) -> Int {
    let mut v: Shape = Circle(k)
    readOnly(&mut v)
}

fn main() -> Unit! {}
`

// The lent cell lives in the caller's frame; the callee stores through it.
func TestMutRefArgWritesBackThroughCell(t *testing.T) {
	ir := emitIR(t, mutRefSrc)
	callee := bodyOf(t, ir, `define ptr @"main.reset"(`)
	if !strings.Contains(callee, "@rt_cellset(") {
		t.Errorf("`s = Circle(k)` does not store through the lent cell:\n%s", callee)
	}
	caller := fnBody(t, ir, "main.replaceLocal")
	checkNoHeap(t, "main.replaceLocal", caller)
	if !strings.Contains(caller, "@rt_cell_init(") || !strings.Contains(caller, "@rt_cell_take(") {
		t.Errorf("the caller does not lend a frame cell and take it back:\n%s", caller)
	}
}

// Wrapping a forwarded borrow again would write the new value back into a temporary instead of the caller's cell.
func TestMutRefForwardsTheSameCell(t *testing.T) {
	body := bodyOf(t, emitIR(t, mutRefSrc), `define ptr @"main.forward"(`)
	if strings.Contains(body, "@rt_cell_init(") || strings.Contains(body, "@rt_cell(") {
		t.Errorf("forwarding a lent borrow builds a second cell:\n%s", body)
	}
}

func TestReadOnlyMutRefBuildsNoCell(t *testing.T) {
	ir := emitIR(t, mutRefSrc)
	for _, fn := range []string{"main.readOnly", "main.readLocal"} {
		body := fnBody(t, ir, fn)
		for _, call := range []string{"@rt_cell_init(", "@rt_cell(", "@rt_cell_take("} {
			if strings.Contains(body, call) {
				t.Errorf("%s builds a cell for a borrow nothing replaces (%s):\n%s", fn, call, body)
			}
		}
	}
}
