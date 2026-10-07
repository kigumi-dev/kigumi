package mir_test

import (
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/testkit"
)

func planned(t *testing.T, src string) *mir.Program {
	t.Helper()
	prog := mir.Build(checkSourceMir(t, src))
	if prog.HasErrors() {
		t.Fatalf("build errors:\n%s", prog.Render())
	}
	passes := []mir.Transform{
		{Name: "plan-rc", Run: mir.PlanRC},
		{Name: "plan-direct", Run: mir.PlanDirect},
	}
	if err := prog.Apply(passes...); err != nil {
		t.Fatalf("passes: %v\n%s", err, prog.Dump())
	}
	return prog
}

func funcNamed(t *testing.T, p *mir.Program, name string) *mir.Func {
	t.Helper()
	for _, f := range p.Funcs {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no function %s", name)
	return nil
}

func countOp(f *mir.Func, op mir.Opcode) int {
	n := 0
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == op {
				n++
			}
		}
	}
	return n
}

const directPointSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn local(x: Int) -> Int {
    let p = Point { x: x, y: x }
    p.x + p.y
}

fn intoArray(x: Int) -> Array[Point] {
    Array.of(Point { x: x, y: x })
}

fn main() -> Unit! {}
`

// A direct aggregate built and read in one body never meets the boxed
// model, so the plan has no crossing to make explicit.
func TestPlanDirectLeavesLocalUseAlone(t *testing.T) {
	f := funcNamed(t, planned(t, directPointSrc), "main.local")
	if n := countOp(f, mir.OpBoxDirect); n != 0 {
		t.Errorf("a record built and read locally boxes %d times:\n%s", n, f.Name)
	}
	direct := 0
	for i := range f.Locals {
		if f.IsDirect(mir.LocalID(i)) {
			direct++
		}
	}
	if direct == 0 {
		t.Error("no local was planned unboxed")
	}
}

// Handing the value to an Array element slot is one explicit crossing.
func TestPlanDirectBoxesAtTheCarrier(t *testing.T) {
	p := planned(t, directPointSrc)
	f := funcNamed(t, p, "main.intoArray")
	if countOp(f, mir.OpBoxDirect) == 0 {
		t.Errorf("storing a direct aggregate into an Array has no explicit box:\n%s", p.Dump())
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op != mir.OpBoxDirect {
				continue
			}
			if !f.IsDirect(in.Args[0]) || f.IsDirect(in.Dst) {
				t.Error("a box does not go from an unboxed local to a carrier")
			}
			if in.Type != f.Locals[in.Args[0]].Type {
				t.Error("a box does not carry the aggregate's own type")
			}
		}
	}
}

// A signature that carries the aggregate returns it unboxed; a body whose
// signature cannot (a closure) boxes once on the way out.
func TestPlanDirectReturnFollowsTheSignature(t *testing.T) {
	p := planned(t, `
type Point = {
    pub x Int
    pub y Int
}

fn mk(x: Int) -> Point { Point { x: x, y: x } }

fn main() -> Unit! {
    let f = (x: Int) => Point { x: x, y: x }
    let p = mk(1)
    print "${p.x} ${f(2).y}"
}
`)
	f := funcNamed(t, p, "main.mk")
	if f.DSig == nil || f.DSig.Ret == 0 {
		t.Fatal("a direct-aggregate result is not in the signature")
	}
	if countOp(f, mir.OpBoxDirect) != 0 {
		t.Error("a signature that carries the aggregate still boxes on return")
	}
	lambda := ""
	for _, g := range p.Funcs {
		if g.DSig == nil && countOp(g, mir.OpBoxDirect) > 0 {
			lambda = g.Name
		}
	}
	if lambda == "" {
		t.Errorf("no body without a direct signature boxes its result:\n%s", p.Dump())
	}
}

// Covers every mir/run fixture; verifyDirect, run through Apply, is the actual crossing check.
func TestPlanDirectCorpus(t *testing.T) {
	std := testkit.LoadStd(t, "../../std")
	var files []string
	for _, glob := range []string{"../../testdata/mir/*.txtar", "../../testdata/run/*.txtar"} {
		fs, _ := filepath.Glob(glob)
		files = append(files, fs...)
	}
	if len(files) == 0 {
		t.Fatal("no fixtures found")
	}
	sawBox, sawDirect := false, false
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			res, _, _ := checkFixture(t, path, std)
			prog := mir.Build(res)
			if prog.HasErrors() {
				t.Skip("fixture expects ownership diagnostics")
			}
			err := prog.Apply(
				mir.Transform{Name: "plan-rc", Run: mir.PlanRC},
				mir.Transform{Name: "plan-direct", Run: mir.PlanDirect},
			)
			if err != nil {
				t.Fatalf("plan-direct: %v\n%s", err, prog.Dump())
			}
			for _, f := range prog.Funcs {
				for i := range f.Locals {
					sawDirect = sawDirect || f.IsDirect(mir.LocalID(i))
				}
				sawBox = sawBox || countOp(f, mir.OpBoxDirect) > 0
			}
		})
	}
	if !sawDirect {
		t.Error("no fixture planned a single unboxed local")
	}
	if !sawBox {
		t.Error("no fixture needed a single crossing, so the insertion step is dead")
	}
}

// A function whose signature carries the aggregate agrees with its callers
// on every parameter, or the two sides disagree about the ABI.
func TestPlanDirectSignatureIsStable(t *testing.T) {
	p := planned(t, directPointSrc+`
fn area(p: Point) -> Int { p.x * p.y }
`)
	f := funcNamed(t, p, "main.area")
	if f.DSig == nil || f.DSig.Param(0) == 0 {
		t.Fatal("a direct-aggregate parameter is not in the signature")
	}
	if got := f.DSig.Param(0); got != f.Locals[f.Params[0]].Type {
		t.Errorf("the signature names %v, the parameter is %v", got, f.Locals[f.Params[0]].Type)
	}
	if !strings.Contains(p.Dump(), "fn main.area") {
		t.Error("the dump lost the function")
	}
}

// A value a body only ever hands to reference slots gains nothing unboxed,
// so the plan leaves it boxed instead of paying a box at every use.
func TestPlanDirectDropsAnAggregateNothingReadsDirectly(t *testing.T) {
	p := planned(t, `
fn count(n: usize) -> usize {
    let mut total: usize = 0
    for i in 0..n {
        total = total + i
    }
    total
}

fn main() -> Unit! {
    print "${count(3)}"
}
`)
	f := funcNamed(t, p, "main.count")
	if n := countOp(f, mir.OpBoxDirect); n != 0 {
		t.Errorf("a range read only through the iterator builtins boxes %d times:\n%s", n, p.Dump())
	}
}

// A plain `self` is a value the callee may hold as a copy, so the signature
// carries it; a `mut self` receiver is the caller's write-back cell.
func TestPlanDirectCarriesAPlainReceiver(t *testing.T) {
	p := planned(t, `
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
`)
	g := funcNamed(t, p, "main.Point.combine")
	if g.DSig == nil || g.DSig.Param(0) == 0 || g.DSig.Param(1) == 0 {
		t.Fatalf("the signature leaves a by-value position out:\n%s", p.Dump())
	}
	f := funcNamed(t, p, "main.touch")
	if n := countOp(f, mir.OpBoxDirect); n != 0 {
		t.Errorf("a call that crosses nothing boxes %d times:\n%s", n, p.Dump())
	}
}

func TestPlanDirectBoxesAMutReceiver(t *testing.T) {
	p := planned(t, `
type Point = {
    pub x Int
    pub mut y Int
}

fn Point.raise(mut self) -> Unit {
    self.y += 1
}

fn touch(n: Int) -> Int {
    let mut p = Point { x: n, y: n + 1 }
    p.raise()
    p.y
}

fn main() -> Unit! {
    print "${touch(1)}"
}
`)
	g := funcNamed(t, p, "main.Point.raise")
	if g.DSig != nil && g.DSig.Param(0) != 0 {
		t.Fatalf("a write-back receiver is carried as a value:\n%s", p.Dump())
	}
	f := funcNamed(t, p, "main.touch")
	if countDirect(f) != 0 {
		t.Errorf("a value lent for a write-back is kept unboxed:\n%s", p.Dump())
	}
	if n := countOp(f, mir.OpBoxDirect); n != 0 {
		t.Errorf("a value boxed from the start crosses %d more times:\n%s", n, p.Dump())
	}
}

func countDirect(f *mir.Func) int {
	n := 0
	for i := range f.Locals {
		if f.IsDirect(mir.LocalID(i)) {
			n++
		}
	}
	return n
}

const directAdtSrc = `
type Shape = Circle(r: Int) | Rect(w: Int, h: Int) | Dot

fn area(n: Int) -> Int {
    let s = if n == 0 { Dot } else { Rect(n, n + 1) }
    match s {
        Circle(r) -> r
        Rect(w, h) -> w * h
        Dot -> 0
    }
}

fn store(n: Int, mut xs: Array[Shape]) -> Unit {
    xs.push(Circle(n))
}
`

func TestPlanDirectBuildsAndMatchesAnAdtInPlace(t *testing.T) {
	p := planned(t, directAdtSrc)
	f := funcNamed(t, p, "main.area")
	if n := countOp(f, mir.OpBoxDirect) + countOp(f, mir.OpUnboxDirect); n != 0 {
		t.Errorf("building and matching an ADT in one body crossed %d times:\n%s", n, p.Dump())
	}
	direct := 0
	for i := range f.Locals {
		if f.IsDirect(mir.LocalID(i)) {
			direct++
		}
	}
	if direct == 0 {
		t.Errorf("no local of main.area stayed unboxed:\n%s", p.Dump())
	}
}

func TestPlanDirectBoxesAnAdtAtTheCarrier(t *testing.T) {
	p := planned(t, directAdtSrc)
	f := funcNamed(t, p, "main.store")
	if n := countOp(f, mir.OpBoxDirect); n != 1 {
		t.Errorf("an ADT handed to an Array crossed %d times, want 1:\n%s", n, p.Dump())
	}
}

const carrierMatchSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn copyAll(xs: &Array[Point]) -> Array[Point] {
    let mut ys = Array.empty[Point]()
    for i in 0..xs.len() {
        if xs.get(i) is Some(p) { ys.push(p) }
    }
    ys
}

fn sumAll(xs: &Array[Point]) -> Int {
    let mut n = 0
    for i in 0..xs.len() {
        if xs.get(i) is Some(p) { n = n + p.x }
    }
    n
}
`

func TestPlanDirectMatchesACarrierInPlace(t *testing.T) {
	p := planned(t, carrierMatchSrc)
	f := funcNamed(t, p, "main.copyAll")
	if n := countOp(f, mir.OpBoxDirect) + countOp(f, mir.OpUnboxDirect); n != 0 {
		t.Errorf("matching a carrier and handing its payload on crossed %d times:\n%s", n, p.Dump())
	}
	f = funcNamed(t, p, "main.sumAll")
	if n := countOp(f, mir.OpBoxDirect); n != 0 {
		t.Errorf("reading a field of a matched payload boxed %d times:\n%s", n, p.Dump())
	}
	if n := countOp(f, mir.OpUnboxDirect); n != 1 {
		t.Errorf("reading a field of a matched payload unboxed %d times, want 1:\n%s", n, p.Dump())
	}
}

const fixedCrossSrc = `
type Tick = Up(n: Int) | Flat

fn pick[T](v: T) -> T { v }

fn fixed() -> Int {
    if pick(Some(1)) is Some(v) { v } else { 0 }
}

fn nested() -> Bool {
    pick(Some(Up(7))) is Some(Up(7))
}

fn fresh(n: Int) -> Int {
    if pick(Some(n)) is Some(v) { v } else { 0 }
}
`

func boxFlags(f *mir.Func) (fixed, fresh int) {
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			switch {
			case in.Op != mir.OpBoxDirect:
			case in.Fixed:
				fixed++
			default:
				fresh++
			}
		}
	}
	return fixed, fresh
}

func TestPlanDirectMarksAFixedCrossing(t *testing.T) {
	p := planned(t, fixedCrossSrc)
	for _, name := range []string{"main.fixed", "main.nested"} {
		if fixed, fresh := boxFlags(funcNamed(t, p, name)); fixed != 1 || fresh != 0 {
			t.Errorf("%s: %d fixed and %d other crossings, want 1 and 0:\n%s", name, fixed, fresh, p.Dump())
		}
	}
	if fixed, fresh := boxFlags(funcNamed(t, p, "main.fresh")); fixed != 0 || fresh != 1 {
		t.Errorf("main.fresh: %d fixed and %d other crossings, want 0 and 1:\n%s", fixed, fresh, p.Dump())
	}
	if !strings.Contains(p.Dump(), "boxdirect fixed(") {
		t.Errorf("the dump does not show the fixed crossing:\n%s", p.Dump())
	}
}

const definedOverRecordSrc = `
type Point = {
    pub x Int
    pub y Int
}

type Marked = Point

fn bump(p: &mut Point, n: Int) -> Unit { p = Point { x: n, y: n + 1 } }

fn marked(n: Int) -> Int {
    let mut m = Marked(Point { x: n * 2, y: n })
    bump(&mut m.value, n + 1)
    m.value.x + m.value.y
}
`

// sem must charge both crossings or the alloc-claim check rejects the plan.
func TestPlanDirectChargesADefinedWrapAndAMutStore(t *testing.T) {
	p := planned(t, definedOverRecordSrc)
	for _, name := range []string{"main.marked", "main.bump"} {
		if n := countOp(funcNamed(t, p, name), mir.OpBoxDirect); n != 1 {
			t.Errorf("%s boxes %d times, want 1:\n%s", name, n, p.Dump())
		}
	}
}
