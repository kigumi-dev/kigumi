package llgen_test

import (
	"strings"
	"testing"
)

// A `mut self` call lends its receiver a cell in the caller's frame, so a
// loop of such calls reaches no allocator and grows no stack per iteration.
func TestMutSelfLoopBuildsNoCell(t *testing.T) {
	ir := emitIR(t, `
type Counter = {
    pub mut n Int
    pub mut tag String
}

fn Counter.bump(mut self, by: Int) {
    self.n = self.n + by
}

fn drive(start: Counter, k: Int) -> Int {
    let mut acc = start
    let mut i = 0
    for i < k {
        acc.bump(i)
        i = i + 1
    }
    acc.n
}

fn main() -> Void! {}
`)
	body := bodyOf(t, ir, `define ptr @"main.drive"(`)
	checkNoHeap(t, "main.drive", body)
	if !strings.Contains(body, "@rt_cell_init(") {
		t.Errorf("the receiver cell is not built in the frame:\n%s", body)
	}
	entry, rest, _ := strings.Cut(body, "\nb0:")
	if !strings.Contains(entry, "alloca [64 x i8]") {
		t.Errorf("the cell's storage is not hoisted to the entry block:\n%s", body)
	}
	if strings.Contains(rest, "alloca [64 x i8]") {
		t.Errorf("a loop body allocates stack per iteration:\n%s", body)
	}
}

// A whole-value `self = ...` stores through that same cell, so the caller
// takes the new value back out of it.
func TestWholeSelfAssignWritesBackThroughCell(t *testing.T) {
	ir := emitIR(t, `
type Shape = Circle(label: String) | Dot

fn Shape.flatten(mut self) {
    self = Dot
}

fn clear(s: Shape) -> Shape {
    let mut out = s
    out.flatten()
    out
}

fn main() -> Void! {}
`)
	callee := bodyOf(t, ir, `define ptr @"main.Shape.flatten"(`)
	if !strings.Contains(callee, "@rt_cellset(") {
		t.Errorf("`self = Dot` does not store through the receiver cell:\n%s", callee)
	}
	caller := bodyOf(t, ir, `define ptr @"main.clear"(`)
	checkNoHeap(t, "main.clear", caller)
	if !strings.Contains(caller, "@rt_cell_take(") {
		t.Errorf("the caller does not take the receiver cell's content back:\n%s", caller)
	}
}
