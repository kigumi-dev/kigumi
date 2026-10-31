package mir_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

func TestVerifyReleaseOnOnePath(t *testing.T) {
	runRuleCases(t, rulesSrc+`
fn Point.choose(self, a: Point, b: Point) -> Point {
    if self.x > self.y { a } else { b }
}
let _ = Point { x: 1, y: 2 }.choose(Point { x: 5, y: 6 }, Point { x: 7, y: 8 })
`, true, []ruleCase{
		{"result viewing a slot one path moved out", "main.Point.choose", "refs", "terminator reads %t6 after its value was released", func(t *testing.T, p *mir.Program, f *mir.Func) {
			b := localNamed(t, f, "b")
			f.Blocks[1].Insts[0].Op = mir.OpMove
			f.Blocks[1].Insts[1] = mir.Inst{Op: mir.OpAlias, Dst: f.Blocks[1].Insts[1].Dst, Args: []mir.LocalID{b}}
			f.Blocks[2].Insts[0].Op = mir.OpMove
			f.Blocks[2].Insts[1] = mir.Inst{Op: mir.OpAlias, Dst: f.Blocks[2].Insts[1].Dst, Args: f.Blocks[2].Insts[1].Args}
		}},
	})
}

func TestVerifyWitnessedValue(t *testing.T) {
	src := rulesSrc + "\nfn forward[T: Show](x: T) -> String { both(x) }\n"
	runRuleCases(t, src, false, []ruleCase{
		{"value for a parameter its witness is for", "main.forward", "calls", "with a witness for T, not of type String", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpCall, 0).Args[0] = newString(f)
		}},
	})
	runRuleCases(t, rulesSrc+"\nlet _ = both(Point { x: 1, y: 2 })\n", false, []ruleCase{
		{"value for a type its bound witness is not for", "main", "calls", "with a witness made for Point, not String", func(t *testing.T, p *mir.Program, f *mir.Func) {
			callOf(t, p, f, "both").Args[0] = newString(f)
		}},
	})
}

func TestVerifyMutReceiverIsACell(t *testing.T) {
	runRuleCases(t, cellRulesSrc+"\nfn peek(p: &mut Point) -> Int { p.y }\n", false, []ruleCase{
		{"borrow of a reference that is no cell", "main.peek", "cellops", "which is no cell, to a mut receiver", func(t *testing.T, p *mir.Program, f *mir.Func) {
			ref := addLocal(f, "r", p.R.Types.Ref(f.Locals[f.Params[0]].Type, true))
			appendInst(f, mir.Inst{Op: mir.OpBorrow, Dst: ref, Args: []mir.LocalID{f.Params[0]}, Index: 1})
			appendInst(f, mir.Inst{Op: mir.OpCall, Dst: addLocal(f, "u", sem.TyUnit), Ent: entityNamed(t, p, sem.EntFn, "bump"), Args: []mir.LocalID{ref}})
		}},
	})
}

func callOf(t *testing.T, p *mir.Program, f *mir.Func, name string) *mir.Inst {
	t.Helper()
	ent := entityNamed(t, p, sem.EntFn, name)
	for bi := range f.Blocks {
		for k := range f.Blocks[bi].Insts {
			if in := &f.Blocks[bi].Insts[k]; in.Op == mir.OpCall && in.Ent == ent {
				return in
			}
		}
	}
	t.Fatalf("%s does not call %s", f.Name, name)
	return nil
}

func TestVerifyVariantsKeepNarrowingThroughReassignment(t *testing.T) {
	for name, src := range map[string]string{
		"test then clear": `
let mut slot: String? = Some("a")
let mut out = Array.of[String]()
for i in 0..4 {
    if slot is Some(s) {
        out.push(s)
        slot = None
    } else {
        slot = Some("n${i}")
    }
}
`,
		"loop head test": `
fn walk(start: Int?) -> Int {
    let mut cur = start
    let mut t = 0
    for cur is Some(c) {
        t += c
        cur = if c > 0 { Some(c - 1) } else { None }
    }
    t
}
let _ = walk(Some(4))
`,
		"match then reassign": `
fn m() -> Int {
    let mut cur = Some(3)
    let mut t = 0
    for i in 0..10 {
        match cur {
            Some(c) -> {
                t += c
                cur = if c > 0 { Some(c - 1) } else { None }
            }
            None -> { t += 1 }
        }
    }
    t
}
let _ = m()
`,
	} {
		t.Run(name, func(t *testing.T) {
			planned(t, src)
		})
	}
}

func TestVerifyClosureSignature(t *testing.T) {
	runRuleCases(t, rulesSrc+`
fn twin() -> Int {
    let a = () => 1
    let b = (x: Int) => x + 1
    let c = () => true
    if c() { a() + b(2) } else { 0 }
}
let _ = twin()
`, false, []ruleCase{
		{"closure of another parameter count", "main.twin", "calls", "yields a function of 0 parameters for", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpClosure, 0).Ent = nth(t, f, mir.OpClosure, 1).Ent
		}},
		{"closure of another result", "main.twin", "calls", "which returns", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpClosure, 0).Ent = nth(t, f, mir.OpClosure, 2).Ent
		}},
	})
}

func TestVerifyIndexOperands(t *testing.T) {
	runRuleCases(t, rulesSrc+`
fn first(a: Array[Int], k: Int) -> Int { a[0] }

fn put(a: &mut Array[Int], v: Int) { a[0] = v }

let _ = first(Array.of(1, 2), 3)
let mut xs = Array.of(1, 2)
put(&mut xs, 3)
`, false, []ruleCase{
		{"index of a scalar", "main.first", "aggregates", "which is no array, string or bytes", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpIndex, 0).Args[0] = f.Params[1]
		}},
		{"index by a string", "main.first", "aggregates", "indexes by String, not a usize", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpIndex, 0).Args[1] = newString(f)
		}},
		{"store of another type", "main.put", "aggregates", "stores String in an array of Int", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpSetIndex, 0).Args[2] = newString(f)
		}},
	})
}

const crossSrc = `
type Point = {
    pub x Int
    pub mut y Int
}

fn Point.bump(mut self) { self.y += 1 }

fn run() -> Int {
    let mut p = Point { x: 1, y: 2 }
    p.bump()
    let copy = p
    p.bump()
    copy.y + p.y
}

let _ = run()
`

func TestVerifyCrossingOfATemporary(t *testing.T) {
	p := planned(t, crossSrc)
	f := funcNamed(t, p, "main.run")
	f.Locals[localNamed(t, f, "p")].Ent = 0
	wantRule(t, p, "refs", "reads %p after its value was released")
}

func TestVerifyArrayBuilds(t *testing.T) {
	p := program(t)
	locals := []mir.Local{{Name: "t0", Type: sem.TyI64}, {Name: "t1", Type: sem.TyString}}
	body := fn("build", sem.TyI64, locals, mir.Block{Insts: []mir.Inst{{Op: mir.OpArray, Dst: 1, Args: []mir.LocalID{0}}}, Term: ret(0)})
	expectRule(t, p, body, "aggregates", "builds String, which is not an array")
}

func TestVerifyCellParameters(t *testing.T) {
	src := `
import task from std/task

type Counter = {
    pub mut n Int
}

async fn Counter.bumpLater(mut self, by: Int) -> Int {
    self.n = self.n + by
    self.n
}

let mut c = Counter { n: 1 }
let got = task.currentThreadExecutor().runToCompletion(c.bumpLater(10))
`
	runRuleCases(t, src, false, []ruleCase{
		{"future passing a value where a cell goes", "main", "calls", "which is no cell, as argument 0 of main.Counter.bumpLater", func(t *testing.T, p *mir.Program, f *mir.Func) {
			nth(t, f, mir.OpFutureNew, 0).Args[1] = nth(t, f, mir.OpNewCell, 0).Args[0]
		}},
	})
}

func TestVerifySymbolCallKeepsItsArity(t *testing.T) {
	runRuleCases(t, rulesSrc+"\nlet mut xs = Array.of(1)\nxs.push(2)\n", false, []ruleCase{
		{"extra argument of a primitive that returns Void", "main", "calls", "which takes", func(t *testing.T, p *mir.Program, f *mir.Func) {
			in := nth(t, f, mir.OpCall, 1)
			in.Args = append(in.Args, in.Args[0])
		}},
	})
}

func TestVerifyCellIsOnlySetToACell(t *testing.T) {
	runRuleCases(t, cellRulesSrc, false, []ruleCase{
		{"cell parameter set to a value", "main.Point.bump", "cellops", "sets the cell %self to a value that is no cell", func(t *testing.T, p *mir.Program, f *mir.Func) {
			appendInst(f, mir.Inst{Op: mir.OpAlias, Dst: f.Params[0], Args: []mir.LocalID{nth(t, f, mir.OpCellGet, 0).Dst}})
		}},
	})
}

func TestVerifyDropInsideALoopOfAnOuterValue(t *testing.T) {
	runRuleCases(t, `
fn count(n: Int) -> Int {
    let mut out = Array.of(0)
    for i in 0..n {
        out.push(i)
    }
    let _ = out.len()
    n
}
let _ = count(3)
`, true, []ruleCase{
		{"outer value dropped inside the loop", "main.count", "refs", "after its value was released", func(t *testing.T, p *mir.Program, f *mir.Func) {
			made := nth(t, f, mir.OpCall, 0).Dst
			last := len(f.Blocks) - 1
			for bi := range f.Blocks {
				if f.Blocks[bi].Term.Op == mir.TermJump && int(f.Blocks[bi].Term.Targets[0]) < bi {
					last = bi
				}
			}
			f.Blocks[last].Insts = append(f.Blocks[last].Insts, mir.Inst{Op: mir.OpDrop, Args: []mir.LocalID{made}})
		}},
	})
}

func TestVerifyEntryTakesNoParameters(t *testing.T) {
	p := built(t, rulesSrc)
	if p.Entry == nil {
		t.Fatal("no entry")
	}
	p.Entry.Params = append(p.Entry.Params, addLocal(p.Entry, "ex", sem.TyI64))
	wantRule(t, p, "entry", "takes Int, not a Host")
}
