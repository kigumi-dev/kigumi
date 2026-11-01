package verify_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

func nth(t *testing.T, f *midir.Func, op midir.Opcode, n int) *midir.Inst {
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
	p := mirfixture.Program(t)
	unit := []midir.Local{{Name: "t0", Type: sem.TyUnit}}
	one := midir.Block{Insts: []midir.Inst{{Op: midir.OpUnit, Dst: 0}}}
	with := func(term midir.Term) *midir.Func {
		b := one
		b.Term = term
		return mirfixture.Fn("term", sem.TyUnit, unit, b)
	}
	f := with(mirfixture.Ret(0))
	f.Params = []midir.LocalID{4}
	expectRule(t, p, f, "structure", "parameter 4 out of range")
	expectRule(t, p, with(midir.Term{Op: midir.TermJump, Args: []midir.LocalID{0}, Targets: []midir.BlockID{0}}), "structure", "jump takes one target")
	expectRule(t, p, with(midir.Term{Op: midir.TermReturn}), "structure", "return takes one argument")
	expectRule(t, p, with(midir.Term{Op: midir.TermUnreachable, Args: []midir.LocalID{0}}), "structure", "unreachable takes nothing")
	expectRule(t, p, with(midir.Term{Op: midir.TermOp(9)}), "structure", "unknown terminator")
	expectRule(t, p, with(mirfixture.Ret(5)), "structure", "terminator argument 5 out of range")
	dst := mirfixture.Fn("dst", sem.TyUnit, unit, midir.Block{Insts: []midir.Inst{{Op: midir.OpUnit, Dst: 3}}, Term: mirfixture.Ret(0)})
	expectRule(t, p, dst, "structure", "destination 3 out of range")
}

func TestVerifyShapeMore(t *testing.T) {
	p := mirfixture.Program(t)
	locals := []midir.Local{{Name: "t0", Type: sem.TyI64}, {Name: "t1", Type: sem.TyString}}
	body := func(in midir.Inst) *midir.Func {
		return mirfixture.Fn("shape", sem.TyI64, locals, midir.Block{Insts: []midir.Inst{in}, Term: mirfixture.Ret(0)})
	}
	expectRule(t, p, body(midir.Inst{Op: midir.OpBuiltin, Dst: 0}), "shape", "needs its text")
	expectRule(t, p, body(midir.Inst{Op: midir.OpShell, Dst: 0, Strs: []string{"$"}}), "shape", "shell has 1 holes for 0 arguments")
	expectRule(t, p, body(midir.Inst{Op: midir.OpBorrow, Dst: 0, Args: []midir.LocalID{1}, Index: 7}), "shape", "borrow index 7")
	expectRule(t, p, body(midir.Inst{Op: midir.OpUnary, Dst: 0, Str: "cast", Args: []midir.LocalID{1}}), "shape", "cast needs its target type")
	expectRule(t, p, body(midir.Inst{Op: midir.Opcode(200), Dst: 0}), "shape", "unknown opcode 200")
	expectRule(t, p, body(midir.Inst{Op: midir.OpUnit, Dst: 0, Fixed: true}), "shape", "marked fixed")
	expectRule(t, p, body(midir.Inst{Op: midir.OpUnit, Dst: 0, Copy: true}), "shape", "marked as a copy")
	expectRule(t, p, body(midir.Inst{Op: midir.OpUnit, Dst: 0, Share: true}), "shape", "marked shared")
	expectRule(t, p, body(midir.Inst{Op: midir.OpUnit, Dst: 0, Async: true}), "shape", "marked async")
	expectRule(t, p, body(midir.Inst{Op: midir.OpUnit, Dst: 0, EndScope: true}), "shape", "scope exit")
	expectRule(t, p, body(midir.Inst{Op: midir.OpUnit, Dst: 0, Scoped: true}), "shape", "marked scoped")
}

func TestVerifyTypesMore(t *testing.T) {
	p := mirfixture.Built(t, rulesSrc)
	tt := p.R.Types
	area := mirfixture.FuncNamed(t, p, "main.area")
	shape := area.Locals[area.Params[0]].Type
	one := midir.Inst{Op: midir.OpConst, Dst: 1, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}
	locals := func(ts ...sem.TypeID) []midir.Local {
		var ls []midir.Local
		for i, ty := range ts {
			ls = append(ls, midir.Local{Name: "t" + string(rune('0'+i)), Type: ty})
		}
		return ls
	}
	check := func(name string, ls []midir.Local, ret sem.TypeID, rule, fragment string, insts ...midir.Inst) {
		t.Run(name, func(t *testing.T) {
			expectRule(t, p, mirfixture.Fn(name, ret, ls, midir.Block{Insts: insts, Term: midir.Term{Op: midir.TermReturn, Args: []midir.LocalID{0}}}), rule, fragment)
		})
	}
	check("isvariant", locals(sem.TyI64, sem.TyI64), sem.TyI64, "types", "yields Bool", midir.Inst{Op: midir.OpConst, Dst: 1, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}, midir.Inst{Op: midir.OpIsVariant, Dst: 0, Args: []midir.LocalID{1}, Ent: entityNamed(t, p, sem.EntVariant, "Dot")})
	check("borrow", locals(sem.TyI64, sem.TyI64), sem.TyI64, "types", "borrow of Int yields Int", one, midir.Inst{Op: midir.OpBorrow, Dst: 0, Args: []midir.LocalID{1}})
	check("variant", locals(sem.TyI64), sem.TyI64, "types", "variant of", midir.Inst{Op: midir.OpVariant, Dst: 0, Ent: entityNamed(t, p, sem.EntVariant, "Dot"), Type: shape})
	check("newcell", locals(sem.TyI64, sem.TyString), sem.TyI64, "types", "cell of String into Int", midir.Inst{Op: midir.OpConst, Dst: 1, Type: sem.TyString, Lit: sem.Literal{Kind: sem.LitString}}, midir.Inst{Op: midir.OpNewCell, Dst: 0, Args: []midir.LocalID{1}})
	check("box", locals(tt.Iface(tt.ErrorEnt(), nil), sem.TyI64), tt.Iface(tt.ErrorEnt(), nil), "types", "box of Int as", midir.Inst{Op: midir.OpConst, Dst: 1, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}}, midir.Inst{Op: midir.OpBox, Dst: 0, Args: []midir.LocalID{1}, Type: sem.TyString})
	check("unit", locals(sem.TyI64), sem.TyI64, "types", "unit into Int", midir.Inst{Op: midir.OpUnit, Dst: 0})
	check("comparison", locals(sem.TyI64, sem.TyI64), sem.TyI64, "types", "comparison == yields Int, not Bool", midir.Inst{Op: midir.OpBinary, Dst: 0, Str: "==", Args: []midir.LocalID{1, 1}})
	check("operator", locals(sem.TyString, sem.TyI64), sem.TyString, "types", "operator + of Int and Int yields String", midir.Inst{Op: midir.OpBinary, Dst: 0, Str: "+", Args: []midir.LocalID{1, 1}})
	check("cast", locals(sem.TyString, sem.TyI64), sem.TyString, "types", "cast to Int into String", midir.Inst{Op: midir.OpUnary, Dst: 0, Str: "cast", Type: sem.TyI64, Args: []midir.LocalID{1}})
	check("fnitem", locals(sem.TyI64), sem.TyI64, "types", "fn item add of", midir.Inst{Op: midir.OpFnItem, Dst: 0, Ent: entityNamed(t, p, sem.EntFn, "add")})
}

func addLocal(f *midir.Func, name string, t sem.TypeID) midir.LocalID {
	f.Locals = append(f.Locals, midir.Local{Name: name, Type: t})
	if len(f.Direct) > 0 {
		f.Direct = append(f.Direct, false)
	}
	if len(f.Slot) > 0 {
		f.Slot = append(f.Slot, -1)
	}
	return midir.LocalID(len(f.Locals) - 1)
}

func appendInst(f *midir.Func, in midir.Inst) {
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

func paramType(f *midir.Func, i int) sem.TypeID { return f.Locals[f.Params[i]].Type }
