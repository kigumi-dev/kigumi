package plan_test

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/devkit/mirfixture"
)

const countedSrc = `
import {Box} from std/alloc

fn grab(s: String) -> Result[String, AllocError] {
    AllocError.wrap(s)
}

fn take(s: String) -> String {
    grab(s) || "refused"
}

fn half(n: Int) -> Result[Int, String] {
    if n % 2 == 1 { return Err("odd") }
    Ok(n / 2)
}

fn quarter(n: Int) -> Result[Int, String] {
    let h = half(n)?
    half(h)
}

fn boxed(n: Int) -> Box[Int] {
    Box.new(n)
}

fn main() -> Void! {}
`

func TestPlanDirectKeepsCountedResultsInline(t *testing.T) {
	p := specialized(t, countedSrc)
	for _, name := range []string{"main.grab", "main.take", "main.half", "main.quarter", "std/alloc.Box.new"} {
		f := mirfixture.FuncNamed(t, p, name)
		if n := mirfixture.CountOp(f, midir.OpBoxDirect) + mirfixture.CountOp(f, midir.OpUnboxDirect); n != 0 {
			t.Errorf("%s crosses a counted Result %d times:\n%s", name, n, p.Dump())
		}
	}
	for _, name := range []string{"main.grab", "std/alloc.Box.new"} {
		tryRef := 0
		for _, blk := range mirfixture.FuncNamed(t, p, name).Blocks {
			for _, in := range blk.Insts {
				if in.Op == midir.OpCall && in.DPrim == opdef.DPTryRef {
					tryRef++
				}
			}
		}
		if tryRef != 1 {
			t.Errorf("%s calls its try op as TryRef %d times, want once:\n%s", name, tryRef, p.Dump())
		}
	}
}

const countedAsyncSrc = `
fn name(n: Int) -> String? {
    if n > 0 { return Some("n") }
    None
}

async fn later(n: Int) -> Int {
    let o = name(n)
    match o {
        Some(s) -> s.len().toInt() || 0
        None -> 0
    }
}

fn main() -> Void! {}
`

func TestPlanDirectBoxesCountedValuesInAsyncFrames(t *testing.T) {
	p := specialized(t, countedAsyncSrc)
	f := mirfixture.FuncNamed(t, p, "main.later")
	for i := range f.Locals {
		if f.IsDirect(midir.LocalID(i)) && p.R.CountedAdt(f.Locals[i].Type) && !f.Locals[i].Cross {
			t.Errorf("async body keeps %s unboxed:\n%s", f.Locals[i].Name, p.Dump())
		}
	}
}

const countedClosureSrc = `
import {Box} from std/alloc

fn mkb(n: Int) -> Box[Int]? { Some(Box.new(n)) }
fn useb(a: Box[Int]?) -> Int { 1 }
fn peek(a: &Box[Int]?) -> Int { if a is Some(b) { b.get() } else { 0 } }

fn main() {
    let f = (a: Box[Int]?) => useb(a)
    let c = mkb(3)
    let h = () => useb(c)
    let k = mkb(4)
    print "${f(mkb(1)) + h() + peek(&k)}"
}
`

func TestPlanDirectMovesCountedClosureParamsOut(t *testing.T) {
	p := specialized(t, countedClosureSrc)
	for _, name := range []string{"main.main.closure1", "main.main.closure2"} {
		if n := mirfixture.CountOp(mirfixture.FuncNamed(t, p, name), midir.OpUnboxDirect); n != 1 {
			t.Errorf("%s unboxes its counted parameter %d times, want once:\n%s", name, n, p.Dump())
		}
	}
	if prm := mirfixture.FuncNamed(t, p, "main.peek").Params[0]; !mirfixture.FuncNamed(t, p, "main.peek").Locals[prm].Borrowed {
		t.Errorf("main.peek takes its lent Option by value without marking it borrowed:\n%s", p.Dump())
	}
}
