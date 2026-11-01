package build_test

import (
	"slices"
	"testing"

	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	midir "kigumi/internal/compiler/mir/ir"
	mirplan "kigumi/internal/compiler/mir/plan"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

const definedLayersSrc = `
type Point = {pub mut x Int, pub mut y Int}
type Loc = Point
type Spot = Loc
type Row = FixedArray[Int, 3]
type Pair = {pub mut p Point, pub mut r Row}
type PairW = Pair

fn write(l: &mut Spot) {
    l.value.value.x = 1
}

fn store() -> Int {
    let mut s = Spot(Loc(Point { x: 1, y: 2 }))
    s.value.value.x = 10
    s.value.value.x
}

fn incI(p: &mut Int) {
    p += 1
}

fn lend() -> Int {
    let mut s = Spot(Loc(Point { x: 1, y: 2 }))
    incI(&mut s.value.value.x)
    s.value.value.x
}

fn lendRow() -> Int {
    let mut r = Row(FixedArray.filled[Int, 3](1))
    incI(&mut r.value[2])
    r.value[2]
}

fn deep() -> Int {
    let mut w = PairW(Pair { p: Point { x: 1, y: 2 }, r: Row(FixedArray.filled[Int, 3](1)) })
    w.value.p.x = 3
    w.value.p.y += 1
    incI(&mut w.value.p.x)
    incI(&mut w.value.r.value[1])
    w.value.p.x
}

fn read(p: &Spot) -> Int {
    p.value.value.x
}

fn main() -> Void! {}
`

func TestDefinedLayersStoreIntoTheNamedLocal(t *testing.T) {
	p := mirfixture.Planned(t, definedLayersSrc)
	for _, name := range []string{"main.store", "main.lend", "main.lendRow", "main.deep"} {
		storesIntoNamedLocal(t, p, mirfixture.FuncNamed(t, p, name))
	}
}

func storesIntoNamedLocal(t *testing.T, p *mir.Program, f *midir.Func) {
	t.Helper()
	found := false
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op != midir.OpSetField && in.Op != midir.OpSetIndex {
				continue
			}
			found = true
			if f.Locals[projectedRoot(f, in.Args[0])].Ent == 0 {
				t.Errorf("%s: store goes to a temporary, not the local:\n%s", f.Name, p.Dump())
			}
		}
	}
	if !found {
		t.Fatalf("%s: no store:\n%s", f.Name, p.Dump())
	}
}

// projectedRoot follows field and index reads back to the local they project from.
func projectedRoot(f *midir.Func, l midir.LocalID) midir.LocalID {
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Dst == l && f.Locals[l].Ent == 0 && (in.Op == midir.OpField || in.Op == midir.OpIndex) {
				return projectedRoot(f, in.Args[0])
			}
		}
	}
	return l
}

func TestDefinedLayersBehindAReferenceShareOneRetag(t *testing.T) {
	p := mirfixture.Planned(t, definedLayersSrc)
	for _, name := range []string{"main.read", "main.write"} {
		if got := mirfixture.CountOp(mirfixture.FuncNamed(t, p, name), midir.OpAlias); got > 1 {
			t.Errorf("%s retags %d times:\n%s", name, got, p.Dump())
		}
	}
}

const definedRefReadSrc = `
type Point = {pub mut x Int, pub mut y Int}
type Loc = Point

fn get(r: &Loc) -> Point { r.value }

fn pick(a: &Loc) -> &Loc { a }

fn viaCall(r: &Loc) -> Int {
    let q = pick(r).value
    q.x
}

fn local() -> Int {
    let mut s = Loc(Point { x: 1, y: 2 })
    let d = (&s).value
    s.value.y = 7
    d.y + s.value.y
}

type View['a] = { pub p &'a Loc }

fn make['a](r: &'a Loc) -> View['a] { View { p: r } }

fn viaField(r: &Loc) -> Int { make(r).p.value.y }

fn viaAlias(r: &Loc) -> Int { { r }.value.x }

fn main() -> Void! {}
`

func TestDefinedReadThroughAReferenceKeepsTheReferent(t *testing.T) {
	p := mirbuild.Program(mirfixture.CheckSourceMir(t, definedRefReadSrc))
	if p.HasErrors() {
		t.Fatalf("build errors:\n%s", p.Render())
	}
	// Not mirfixture.Planned(): the verifier's verdict is reported after these checks so they fail on their own.
	perr := mirplan.Apply(p, mirplan.Transform{Name: "plan-rc", Run: mirplan.PlanRC}, mirplan.Transform{Name: "plan-direct", Run: mirplan.PlanDirect})
	handsOver := map[string]bool{"main.get": false, "main.local": false, "main.viaCall": true, "main.viaField": true, "main.viaAlias": false}
	for name, want := range handsOver {
		f := mirfixture.FuncNamed(t, p, name)
		crossed := 0
		for _, blk := range f.Blocks {
			for i, in := range blk.Insts {
				if in.Op != midir.OpUnboxDirect || p.R.Types.Kind(f.Locals[in.Args[0]].Type) != sem.KRef {
					continue
				}
				crossed++
				if use := nextUse(blk.Insts[i+1:], in.Dst); use == nil || use.Op != midir.OpAlias {
					t.Errorf("%s: the crossing out of %s is not read through an alias:\n%s", name, f.Locals[in.Args[0]].Name, p.Dump())
				}
				if got := p.CrossHandsOver(f, &blk.Insts[i]); got != want {
					t.Errorf("%s: the crossing out of %s hands over its count: %v, want %v:\n%s", name, f.Locals[in.Args[0]].Name, got, want, p.Dump())
				}
			}
		}
		if crossed == 0 {
			t.Errorf("%s: no crossing out of a reference:\n%s", name, p.Dump())
		}
	}
	if perr != nil {
		t.Errorf("passes: %v", perr)
	}
}

func nextUse(insts []midir.Inst, l midir.LocalID) *midir.Inst {
	for i := range insts {
		if slices.Contains(insts[i].Args, l) {
			return &insts[i]
		}
	}
	return nil
}
