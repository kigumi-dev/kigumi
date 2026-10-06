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

// A direct signature covering one parameter says nothing about the rest:
// a lent receiver is a crossing like any other, and MIR names it.
func TestPlanDirectBoxesAPositionOutsideTheSignature(t *testing.T) {
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
	if g.DSig == nil || g.DSig.Param(0) != 0 || g.DSig.Param(1) == 0 {
		t.Fatalf("the signature does not carry `other` while leaving the receiver out:\n%s", p.Dump())
	}
	f := funcNamed(t, p, "main.touch")
	if n := countOp(f, mir.OpBoxDirect); n != 1 {
		t.Errorf("the lent receiver crossing is %d explicit boxes, want 1:\n%s", n, p.Dump())
	}
}
