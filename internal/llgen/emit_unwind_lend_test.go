package llgen_test

import (
	"strings"
	"testing"
)

const lentCaptureSrc = `
type Res = resource {
    pub id Int
}

fn run(f: fn(Int) -> Int) -> Int {
    f(1)
}

fn explode() -> Int {
    None.expect("boom")
}

fn viaBorrow(x: &Res) -> Int {
    run((n: Int) => x.id + n + explode())
}

type Pt = {
    pub x Int
}

fn Pt.read(self, n: Int) -> Int {
    self.x + n + explode()
}

fn viaMethod(p: Pt) -> Int {
    run(p.read)
}

fn main() -> Unit! {}
`

func TestLentCaptureIsBorrowedFromTheEnvironment(t *testing.T) {
	ir := emitIR(t, lentCaptureSrc)
	adapter := bodyOf(t, ir, `define ptr @"main.viaBorrow.closure1$adapter"(`)
	if !strings.Contains(adapter, "@rt_iter_at(ptr %env, i64 0, i32 1)") {
		t.Errorf("the borrowed capture must be read in place:\n%s", adapter)
	}
	if strings.Contains(adapter, "@rt_release(") {
		t.Errorf("nothing was retained for the borrowed capture, so nothing is released after the call:\n%s", adapter)
	}
}

func TestMethodValueBorrowsItsReceiver(t *testing.T) {
	ir := emitIR(t, lentCaptureSrc)
	adapter := bodyOf(t, ir, `define ptr @"main.Pt.read$methodvalue"(`)
	if !strings.Contains(adapter, "@rt_iter_at(ptr %env, i64 0, i32 1)") || strings.Contains(adapter, "@rt_release(") {
		t.Errorf("the receiver must be borrowed from the environment and never released:\n%s", adapter)
	}
}
