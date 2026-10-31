package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

func wantRule(t *testing.T, p *mir.Program, rule, fragment string) {
	t.Helper()
	err := p.Verify()
	ve, ok := err.(*mir.Error)
	if !ok || ve.Rule != rule || !strings.Contains(ve.Msg, fragment) {
		t.Fatalf("got %v, want rule %s mentioning %q", err, rule, fragment)
	}
}

func nth(t *testing.T, f *mir.Func, op mir.Opcode, n int) *mir.Inst {
	t.Helper()
	for bi := range f.Blocks {
		for k := range f.Blocks[bi].Insts {
			if in := &f.Blocks[bi].Insts[k]; in.Op == op {
				if n == 0 {
					return in
				}
				n--
			}
		}
	}
	t.Fatalf("%s has no such %s", f.Name, op)
	return nil
}

func entityNamed(t *testing.T, p *mir.Program, kind sem.EntityKind, name string) sem.EntityID {
	t.Helper()
	for i := range p.R.Entities {
		if e := &p.R.Entities[i]; e.Kind == kind && e.Name == name && p.R.Packages[e.Pkg].Path == "main" {
			return sem.EntityID(i)
		}
	}
	t.Fatalf("no %d entity %s", kind, name)
	return 0
}

func built(t *testing.T, src string) *mir.Program {
	t.Helper()
	p := mir.Build(checkSourceMir(t, src))
	if p.HasErrors() {
		t.Fatalf("build errors:\n%s", p.Render())
	}
	if err := p.Verify(); err != nil {
		t.Fatalf("the base program does not verify: %v", err)
	}
	return p
}

const rulesSrc = `
type Shape = Circle(r: Float) | Rect(w: Float, h: Float) | Dot

type Point = {
    pub x Int
    pub mut y Int
}

interface Show {
    fn show(self) -> String
}

fn Point.show(self) -> String { "p" }

fn add(a: Int, b: Int) -> Int { a + b }

fn area(s: Shape) -> Float {
    match s {
        Circle(r) -> 3.0 * r * r
        Rect { w, h } -> w * h
        Dot -> 0.0
    }
}

fn shift(p: Point, dx: Int) -> Point { Point { x: p.x + dx, y: p.y } }

fn pick(xs: Array[Int]) -> Int {
    if xs.get(0) is Some(v) { v } else { 0 }
}

fn run(f: fn(Int) -> Int, n: Int) -> Int { f(n) }

fn sum(xs: Array[Int]) -> Int {
    let mut n = 0
    for x in xs { n += x }
    n
}

fn counter() -> Int {
    let mut n = 0
    let inc = () => { n += 1 }
    inc()
    n
}

fn both[T: Show](x: T) -> String { x.show() }

fn lend(mut p: Point) { p.y += 1 }

fn mk() -> Shape { Rect(1.0, 2.0) }

fn greet(n: String) -> String { "hi ${n}" }

fn keep(xs: Array[String], s: String) -> Array[String] {
    let mut out = xs
    out.push(s)
    out
}

fn grab(o: String?) -> String? {
    let mut m = o
    m.take()
}

fn Point.total(self) -> Int { self.x + self.y }

type Named = {
    pub items Array[Int]
}

fn Named.label(self) -> Int { 1 }

fn Point.moved(self, dx: Int) -> Point { Point { x: self.x + dx, y: self.y } }

fn eat(s: String) -> Int { 1 }

fn feed(s: String) -> Int { eat(s) }

fn arr() -> Array[Int] { Array.of(1, 2) }

let _ = add(1, 2)
`

func TestVerifyStructureMore(t *testing.T) {
	p := program(t)
	unit := []mir.Local{{Name: "t0", Type: sem.TyUnit}}
	one := mir.Block{Insts: []mir.Inst{{Op: mir.OpUnit, Dst: 0}}}
	with := func(term mir.Term) *mir.Func {
		b := one
		b.Term = term
		return fn("term", sem.TyUnit, unit, b)
	}
	f := with(ret(0))
	f.Params = []mir.LocalID{4}
	expectRule(t, p, f, "structure", "parameter 4 out of range")
	expectRule(t, p, with(mir.Term{Op: mir.TermJump, Args: []mir.LocalID{0}, Targets: []mir.BlockID{0}}), "structure", "jump takes one target")
	expectRule(t, p, with(mir.Term{Op: mir.TermReturn}), "structure", "return takes one argument")
	expectRule(t, p, with(mir.Term{Op: mir.TermUnreachable, Args: []mir.LocalID{0}}), "structure", "unreachable takes nothing")
	expectRule(t, p, with(mir.Term{Op: mir.TermOp(9)}), "structure", "unknown terminator")
	expectRule(t, p, with(ret(5)), "structure", "terminator argument 5 out of range")
	dst := fn("dst", sem.TyUnit, unit, mir.Block{Insts: []mir.Inst{{Op: mir.OpUnit, Dst: 3}}, Term: ret(0)})
	expectRule(t, p, dst, "structure", "destination 3 out of range")
}

func TestVerifyShapeMore(t *testing.T) {
	p := program(t)
	locals := []mir.Local{{Name: "t0", Type: sem.TyI64}, {Name: "t1", Type: sem.TyString}}
	body := func(in mir.Inst) *mir.Func {
		return fn("shape", sem.TyI64, locals, mir.Block{Insts: []mir.Inst{in}, Term: ret(0)})
	}
	expectRule(t, p, body(mir.Inst{Op: mir.OpBuiltin, Dst: 0}), "shape", "needs its text")
	expectRule(t, p, body(mir.Inst{Op: mir.OpShell, Dst: 0, Strs: []string{"$"}}), "shape", "shell has 1 holes for 0 arguments")
	expectRule(t, p, body(mir.Inst{Op: mir.OpBorrow, Dst: 0, Args: []mir.LocalID{1}, Index: 7}), "shape", "borrow index 7")
	expectRule(t, p, body(mir.Inst{Op: mir.OpUnary, Dst: 0, Str: "cast", Args: []mir.LocalID{1}}), "shape", "cast needs its target type")
	expectRule(t, p, body(mir.Inst{Op: mir.Opcode(200), Dst: 0}), "shape", "unknown opcode 200")
	expectRule(t, p, body(mir.Inst{Op: mir.OpUnit, Dst: 0, Fixed: true}), "shape", "marked fixed")
	expectRule(t, p, body(mir.Inst{Op: mir.OpUnit, Dst: 0, Copy: true}), "shape", "marked as a copy")
	expectRule(t, p, body(mir.Inst{Op: mir.OpUnit, Dst: 0, Share: true}), "shape", "marked shared")
	expectRule(t, p, body(mir.Inst{Op: mir.OpUnit, Dst: 0, Async: true}), "shape", "marked async")
	expectRule(t, p, body(mir.Inst{Op: mir.OpUnit, Dst: 0, EndScope: true}), "shape", "scope exit")
	expectRule(t, p, body(mir.Inst{Op: mir.OpUnit, Dst: 0, Scoped: true}), "shape", "marked scoped")
}

func TestVerifyTypesMore(t *testing.T) {
	p := built(t, rulesSrc)
	tt := p.R.Types
	area := funcNamed(t, p, "main.area")
	shape := area.Locals[area.Params[0]].Type
	one := mir.Inst{Op: mir.OpConst, Dst: 1, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}
	locals := func(ts ...sem.TypeID) []mir.Local {
		var ls []mir.Local
		for i, ty := range ts {
			ls = append(ls, mir.Local{Name: "t" + string(rune('0'+i)), Type: ty})
		}
		return ls
	}
	check := func(name string, ls []mir.Local, ret sem.TypeID, rule, fragment string, insts ...mir.Inst) {
		t.Run(name, func(t *testing.T) {
			expectRule(t, p, fn(name, ret, ls, mir.Block{Insts: insts, Term: mir.Term{Op: mir.TermReturn, Args: []mir.LocalID{0}}}), rule, fragment)
		})
	}
	check("isvariant", locals(sem.TyI64, sem.TyI64), sem.TyI64, "types", "yields Bool", mir.Inst{Op: mir.OpConst, Dst: 1, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}, mir.Inst{Op: mir.OpIsVariant, Dst: 0, Args: []mir.LocalID{1}, Ent: entityNamed(t, p, sem.EntVariant, "Dot")})
	check("borrow", locals(sem.TyI64, sem.TyI64), sem.TyI64, "types", "borrow of Int yields Int", one, mir.Inst{Op: mir.OpBorrow, Dst: 0, Args: []mir.LocalID{1}})
	check("variant", locals(sem.TyI64), sem.TyI64, "types", "variant of", mir.Inst{Op: mir.OpVariant, Dst: 0, Ent: entityNamed(t, p, sem.EntVariant, "Dot"), Type: shape})
	check("newcell", locals(sem.TyI64, sem.TyString), sem.TyI64, "types", "cell of String into Int", mir.Inst{Op: mir.OpConst, Dst: 1, Type: sem.TyString, Lit: sem.Literal{Kind: sem.LitString}}, mir.Inst{Op: mir.OpNewCell, Dst: 0, Args: []mir.LocalID{1}})
	check("box", locals(tt.Iface(tt.ErrorEnt(), nil), sem.TyI64), tt.Iface(tt.ErrorEnt(), nil), "types", "box of Int as", mir.Inst{Op: mir.OpConst, Dst: 1, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}, mir.Inst{Op: mir.OpBox, Dst: 0, Args: []mir.LocalID{1}, Type: sem.TyString})
	check("unit", locals(sem.TyI64), sem.TyI64, "types", "unit into Int", mir.Inst{Op: mir.OpUnit, Dst: 0})
	check("comparison", locals(sem.TyI64, sem.TyI64), sem.TyI64, "types", "comparison == yields Int, not Bool", mir.Inst{Op: mir.OpBinary, Dst: 0, Str: "==", Args: []mir.LocalID{1, 1}})
	check("operator", locals(sem.TyString, sem.TyI64), sem.TyString, "types", "operator + of Int and Int yields String", mir.Inst{Op: mir.OpBinary, Dst: 0, Str: "+", Args: []mir.LocalID{1, 1}})
	check("cast", locals(sem.TyString, sem.TyI64), sem.TyString, "types", "cast to Int into String", mir.Inst{Op: mir.OpUnary, Dst: 0, Str: "cast", Type: sem.TyI64, Args: []mir.LocalID{1}})
	check("fnitem", locals(sem.TyI64), sem.TyI64, "types", "fn item add of", mir.Inst{Op: mir.OpFnItem, Dst: 0, Ent: entityNamed(t, p, sem.EntFn, "add")})
}

func addLocal(f *mir.Func, name string, t sem.TypeID) mir.LocalID {
	f.Locals = append(f.Locals, mir.Local{Name: name, Type: t})
	if len(f.Direct) > 0 {
		f.Direct = append(f.Direct, false)
	}
	if len(f.Slot) > 0 {
		f.Slot = append(f.Slot, -1)
	}
	return mir.LocalID(len(f.Locals) - 1)
}

func appendInst(f *mir.Func, in mir.Inst) {
	f.Blocks[0].Insts = append(f.Blocks[0].Insts, in)
}

func anyEntity(t *testing.T, p *mir.Program, kind sem.EntityKind, name string) sem.EntityID {
	t.Helper()
	for i := range p.R.Entities {
		if e := &p.R.Entities[i]; e.Kind == kind && e.Name == name {
			return sem.EntityID(i)
		}
	}
	t.Fatalf("no %d entity %s", kind, name)
	return 0
}

func paramType(f *mir.Func, i int) sem.TypeID { return f.Locals[f.Params[i]].Type }
