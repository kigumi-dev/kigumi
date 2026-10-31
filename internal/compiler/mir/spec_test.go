package mir_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/lang/diag"
)

func specialized(t *testing.T, src string) *mir.Program {
	t.Helper()
	prog := mir.Build(checkSourceMir(t, src))
	if prog.HasErrors() {
		t.Fatalf("build errors:\n%s", prog.Render())
	}
	passes := []mir.Transform{
		{Name: "plan-rc", Run: mir.PlanRC},
		{Name: "specialize", Run: mir.Specialize},
		{Name: "plan-direct", Run: mir.PlanDirect},
	}
	if err := prog.Apply(passes...); err != nil {
		t.Fatalf("passes: %v\n%s", err, prog.Dump())
	}
	return prog
}

func instances(p *mir.Program) []string {
	var out []string
	for _, f := range p.Funcs {
		if _, ok := p.R.InstanceOf(f.Ent); ok {
			out = append(out, f.Name)
		}
	}
	return out
}

func calleeOf(t *testing.T, p *mir.Program, f *midir.Func, name string) *midir.Func {
	t.Helper()
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpCall && p.R.Entity(in.Ent).Name == name {
				return p.ByEnt[in.Ent]
			}
		}
	}
	t.Fatalf("%s calls no %s:\n%s", f.Name, name, p.Dump())
	return nil
}

const specPointSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn identity[T](x: T) -> T { x }

fn store[T](x: T) -> Array[T] {
    let mut out = Array.empty[T]()
    out.push(x)
    out
}

fn run(x: Int) -> Int {
    let p = identity(Point { x: x, y: x })
    let n = identity(x)
    p.x + n
}

fn keep(x: Int) -> Int { store(Point { x: x, y: x }).len().toInt() || 0 }

fn main() -> Void! {}
`

func TestSpecializeGivesADirectRecordItsOwnBody(t *testing.T) {
	p := specialized(t, specPointSrc)
	spec := funcNamed(t, p, "main.identity[main.Point]")
	point := spec.Locals[spec.Params[0]].Type
	if spec.DSig.Param(0) != point || spec.DSig.Ret != point {
		t.Errorf("the instantiation does not carry the record unboxed: %+v", spec.DSig)
	}
	run := funcNamed(t, p, "main.run")
	if n := countOp(run, midir.OpBoxDirect) + countOp(run, midir.OpUnboxDirect); n != 0 {
		t.Errorf("the caller still crosses %d times:\n%s", n, p.Dump())
	}
	if n := countOp(spec, midir.OpBoxDirect) + countOp(spec, midir.OpUnboxDirect); n != 0 {
		t.Errorf("the instantiation crosses %d times", n)
	}
}

func TestSpecializeGivesAReturnedScalarItsOwnBody(t *testing.T) {
	p := specialized(t, specPointSrc)
	if got := strings.Join(instances(p), " "); got != "main.identity[main.Point] main.identity[Int]" {
		t.Errorf("instantiations = %q, want the record one and the Int one", got)
	}
	if f := calleeOf(t, p, funcNamed(t, p, "main.run"), "identity"); f == nil || f.Name != "main.identity[main.Point]" && f.Name != "main.identity[Int]" {
		t.Errorf("unexpected callee %v", f)
	}
}

func TestSpecializeKeepsTheSharedBodyWhereTheBoxWouldMoveIn(t *testing.T) {
	p := specialized(t, specPointSrc)
	keep := funcNamed(t, p, "main.keep")
	if f := calleeOf(t, p, keep, "store"); f.Name != "main.store" {
		t.Errorf("store is called through %s, want the shared body", f.Name)
	}
	if countOp(keep, midir.OpBoxDirect) != 1 {
		t.Errorf("want the one box at the call:\n%s", p.Dump())
	}
}

func TestSpecializeCallsTheWitnessImplementation(t *testing.T) {
	p := specialized(t, `
interface Measured {
    fn size(self) -> Int
}

type Point = {
    pub x Int
    pub y Int
}

fn Point.size(self) -> Int { self.x + self.y }

fn measure[T: Measured](x: T) -> Int { x.size() * 2 }

fn run(x: Int) -> Int { measure(Point { x: x, y: 1 }) }

fn main() -> Void! {}
`)
	spec := funcNamed(t, p, "main.measure[main.Point]")
	if countOp(spec, midir.OpCallValue) != 0 {
		t.Errorf("the witness is still called through its value:\n%s", p.Dump())
	}
	if f := calleeOf(t, p, spec, "size"); f.Name != "main.Point.size" {
		t.Errorf("size resolves to %s", f.Name)
	}
	if n := countOp(spec, midir.OpBoxDirect); n != 0 {
		t.Errorf("the receiver is boxed %d times", n)
	}
}

func TestSpecializeRevertsACallThatWouldBoxTheResult(t *testing.T) {
	p := specialized(t, `
type Tag = Up(n: Int) | Flat

fn pick[T](x: T) -> T { x }

fn into(mut xs: Array[Tag]) { xs.push(pick(Up(3))) }

fn moved(x: Int, mut xs: Array[Tag]) { xs.push(pick(Up(x))) }

fn read(x: Int) -> Bool { pick(Up(x)) is Up(3) }

fn main() -> Void! {}
`)
	for name, want := range map[string]string{"main.into": "main.pick", "main.moved": "main.pick[main.Tag]", "main.read": "main.pick[main.Tag]"} {
		if f := calleeOf(t, p, funcNamed(t, p, name), "pick"); f.Name != want {
			t.Errorf("%s calls %s, want %s", name, f.Name, want)
		}
	}
}

func TestSpecializeRecursionSharesOneBody(t *testing.T) {
	p := specialized(t, `
type Point = {
    pub x Int
    pub y Int
}

fn down[T](x: T, n: Int) -> T {
    if n == 0 { x } else { down(x, n - 1) }
}

fn run(n: Int) -> Int { down(Point { x: n, y: n }, n).x }

fn main() -> Void! {}
`)
	spec := funcNamed(t, p, "main.down[main.Point]")
	if f := calleeOf(t, p, spec, "down"); f != spec {
		t.Errorf("the recursive call goes to %s", f.Name)
	}
}

func TestSpecializeWarnsOnRunawayGrowth(t *testing.T) {
	p := specialized(t, `
type Box[T] = {
    pub v T
}

fn Box[T: Copy].nest(self, n: Int) -> Int {
    if n == 0 { 0 } else { 1 + Box { v: Box { v: self.v } }.nest(n - 1) }
}

fn run(n: Int) -> Int { Box { v: n }.nest(n) + Box { v: (n, n) }.nest(n) }

fn main() -> Void! {}
`)
	if len(p.Diags) != 1 || p.Diags[0].Diag.Code != "W1710" || p.Diags[0].Diag.Severity != diag.Warning {
		t.Fatalf("diagnostics = %+v, want one W1710 warning", p.Diags)
	}
	if msg := p.Diags[0].Diag.Msg; !strings.Contains(msg, "`nest`") || !strings.Contains(msg, "Box[main.Box[main.Box[Tuple2[Int,Int]]]]") {
		t.Errorf("message %q", msg)
	}
}

func TestSpecializeFollowsALongChainOfGenerics(t *testing.T) {
	const steps = 80
	var src strings.Builder
	src.WriteString("type Point = {\n    pub x Int\n    pub y Int\n}\n\n")
	for i := 1; i < steps; i++ {
		fmt.Fprintf(&src, "fn step%d[T](x: T) -> T { step%d(x) }\n", i, i+1)
	}
	fmt.Fprintf(&src, "fn step%d[T](x: T) -> T { x }\n\n", steps)
	src.WriteString("fn run(n: Int) -> Int { step1(Point { x: n, y: n }).y }\n\nfn main() -> Void! {}\n")
	p := specialized(t, src.String())
	if len(p.Diags) != 0 {
		t.Fatalf("diagnostics = %+v, want none", p.Diags)
	}
	last := funcNamed(t, p, fmt.Sprintf("main.step%d[main.Point]", steps))
	if n := countOp(last, midir.OpBoxDirect) + countOp(last, midir.OpUnboxDirect); n != 0 {
		t.Errorf("the last step crosses %d times", n)
	}
}

func TestSpecializeRevertsSitesCopiedFromTheGenericBody(t *testing.T) {
	p := specialized(t, `
type Point = {
    pub x Int
    pub y Int
}

fn inner[T](x: T) -> Array[T] { Array.of(x) }

fn outer[T](x: T, p: Point) -> T {
    let xs = inner(p)
    x
}

fn run(n: Int) -> Int { outer(Point { x: n, y: n }, Point { x: 1, y: 2 }).y }

fn main() -> Void! {}
`)
	for _, name := range []string{"main.outer", "main.outer[main.Point]"} {
		if f := calleeOf(t, p, funcNamed(t, p, name), "inner"); f.Name != "main.inner" {
			t.Errorf("%s calls %s, want the shared body", name, f.Name)
		}
	}
}

func TestVerifyRejectsACallToAnInstantiationWithoutABody(t *testing.T) {
	p := specialized(t, specPointSrc)
	delete(p.ByEnt, funcNamed(t, p, "main.identity[main.Point]").Ent)
	ve, ok := p.Verify().(*mir.Error)
	if !ok || ve.Rule != "entities" || !strings.Contains(ve.Msg, "not an instantiation with a body") {
		t.Fatalf("got %v, want an entities error", ve)
	}
}

func TestBuildSkipsMintedInstantiations(t *testing.T) {
	p := specialized(t, specPointSrc)
	again := mir.Build(p.R)
	for _, f := range again.Funcs {
		if _, minted := p.R.InstanceOf(f.Ent); minted {
			t.Errorf("the instantiation %s is lowered as a body of its own", f.Name)
		}
	}
	if fresh := mir.Build(checkSourceMir(t, specPointSrc)); len(again.Funcs) != len(fresh.Funcs) {
		t.Errorf("a second build has %d functions, a fresh one %d", len(again.Funcs), len(fresh.Funcs))
	}
}

func TestSpecializeKeepsAWideRecursionShared(t *testing.T) {
	p := specialized(t, `
type Wide = {
    pub a Int
    pub b Int
    pub c Int
}

type Point = {
    pub x Int
    pub y Int
}

fn down[T](x: T, n: Int) -> T {
    if n == 0 { x } else { down(x, n - 1) }
}

fn isEven[T](x: T, n: Int) -> Bool {
    if n == 0 { true } else { isOdd(x, n - 1) }
}

fn isOdd[T](x: T, n: Int) -> Bool {
    if n == 0 { false } else { isEven(x, n - 1) }
}

fn once[T](x: T) -> T { x }

fn run(n: Int) -> Int {
    let w = Wide { a: n, b: n, c: n }
    let even = if isEven(w, n) { 1 } else { 0 }
    down(w, n).c + down(Point { x: n, y: n }, n).y + once(w).c + even
}

fn main() -> Void! {}
`)
	got := instances(p)
	sort.Strings(got)
	if want := "main.down[main.Point] main.once[main.Wide]"; strings.Join(got, " ") != want {
		t.Errorf("instantiations = %q, want %q", strings.Join(got, " "), want)
	}
}

func TestSpecializeKeepsAWideRecursionForANoallocCaller(t *testing.T) {
	p := specialized(t, `
type Wide = {
    pub a Int
    pub b Int
    pub c Int
}

fn down[T](x: T, n: Int) -> T {
    if n == 0 { x } else { down(x, n - 1) }
}

fn outer[T](x: T, n: Int) -> T { down(x, n) }

fn isEven[T](x: T, n: Int) -> Bool {
    if n == 0 { true } else { isOdd(x, n - 1) }
}

fn isOdd[T](x: T, n: Int) -> Bool {
    if n == 0 { false } else { isEven(x, n - 1) }
}

noalloc fn run(w: Wide, n: Int) -> Int { down(w, n).c }

noalloc fn nested(w: Wide, n: Int) -> Int { outer(w, n).c }

noalloc fn parity(w: Wide, n: Int) -> Bool { isEven(w, n) }

fn main() -> Void! {}
`)
	got := instances(p)
	sort.Strings(got)
	if want := "main.down[main.Wide] main.isEven[main.Wide] main.isOdd[main.Wide] main.outer[main.Wide]"; strings.Join(got, " ") != want {
		t.Errorf("instantiations = %q, want %q", strings.Join(got, " "), want)
	}
	for _, name := range []string{"main.run", "main.nested", "main.parity"} {
		if f := funcNamed(t, p, name); countOp(f, midir.OpBoxDirect) != 0 {
			t.Errorf("%s still boxes:\n%s", name, p.Dump())
		}
	}
}

func fnItemTargets(p *mir.Program, f *midir.Func) []string {
	var out []string
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpFnItem {
				out = append(out, p.ByEnt[in.Ent].Name)
			}
		}
	}
	return out
}

func TestSpecializeFollowsAGenericFnValue(t *testing.T) {
	p := specialized(t, `
type Point = {
    pub x Int
    pub y Int
}

fn identity[T](x: T) -> T { x }

fn run(x: Int) -> Int {
    let f = identity[Point]
    let g = identity[Int]
    f(Point { x: x, y: x }).x + g(x)
}

fn main() -> Void! {}
`)
	got := fnItemTargets(p, funcNamed(t, p, "main.run"))
	if want := []string{"main.identity[main.Point]", "main.identity"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("fn items name %v, want %v\n%s", got, want, p.Dump())
	}
}
