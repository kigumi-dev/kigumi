package llgen_test

import (
	"testing"

	"kigumi/internal/llgen"
	"kigumi/internal/mir"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

const conformSealSrc = `
interface Grower {
    fn grow(mut self, k: Int)
    fn size(self) -> Int
}

type Shape = Circle(r: Int) | Dot

fn Shape.grow(mut self, k: Int) {
    self = Circle(self.size() + k)
}

fn Shape.size(self) -> Int {
    match self {
        Circle(r) -> r
        Dot -> 0
    }
}

type Box = {
    pub mut n Int
}

fn Box.grow(mut self, k: Int) {
    self = Box { n: self.n + k }
}

fn Box.size(self) -> Int {
    self.n
}

fn growStatic[G: Grower](g: &mut G, k: Int) -> Int {
    g.grow(k)
    g.size()
}

fn growDyn(g: &mut Grower, k: Int) -> Int {
    g.grow(k)
    g.size()
}

fn main() -> Void! {
    let mut s: Shape = Circle(1)
    let mut b = Box { n: 2 }
    print "${growStatic(&mut s, 1)} ${growStatic(&mut b, 2)}"
    let mut d: Grower = Dot
    print "${growDyn(&mut d, 3)}"
}
`

// A late conformance query would mean a pair resolveRefCells sealed the witness table without ever seeing.
func TestNoConformanceQueryAfterSeal(t *testing.T) {
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(conformSealSrc)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.Check(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)})
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}, mir.Transform{Name: "plan-direct", Run: mir.PlanDirect}); err != nil {
		t.Fatal(err)
	}
	llgen.Emit(prog)
	if n := res.ConformMissesAfterSeal(); n != 0 {
		t.Errorf("%d conformance queries first asked after resolveRefCells sealed the witness table", n)
	}
}
