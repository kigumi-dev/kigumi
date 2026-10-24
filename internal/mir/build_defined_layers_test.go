package mir_test

import (
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

const definedLayersSrc = `
type Point = {pub mut x Int, pub mut y Int}
type Loc = Point
type Spot = Loc

fn write(l: &mut Spot) {
    l.value.value.x = 1
}

fn store() -> Int {
    let mut s = Spot(Loc(Point { x: 1, y: 2 }))
    s.value.value.x = 10
    s.value.value.x
}

fn read(p: &Spot) -> Int {
    p.value.value.x
}

fn main() -> Void! {}
`

func TestDefinedLayersStoreIntoTheNamedLocal(t *testing.T) {
	p := planned(t, definedLayersSrc)
	f := funcNamed(t, p, "main.store")
	found := false
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op != mir.OpSetField {
				continue
			}
			found = true
			if f.Locals[in.Args[0]].Ent == 0 {
				t.Errorf("store goes to a temporary, not the local:\n%s", p.Dump())
			}
		}
	}
	if !found {
		t.Fatalf("no store:\n%s", p.Dump())
	}
}

func TestDefinedLayersBehindAReferenceShareOneRetag(t *testing.T) {
	p := planned(t, definedLayersSrc)
	for _, name := range []string{"main.read", "main.write"} {
		if got := countOp(funcNamed(t, p, name), mir.OpAlias); got > 1 {
			t.Errorf("%s retags %d times:\n%s", name, got, p.Dump())
		}
	}
}

const definedRefReadSrc = `
type Point = {pub mut x Int, pub mut y Int}
type Loc = Point

fn get(r: &Loc) -> Point { r.value }

fn local() -> Int {
    let mut s = Loc(Point { x: 1, y: 2 })
    let d = (&s).value
    s.value.y = 7
    d.y + s.value.y
}

fn main() -> Void! {}
`

func TestDefinedReadThroughAReferenceKeepsTheReferent(t *testing.T) {
	p := planned(t, definedRefReadSrc)
	for _, name := range []string{"main.get", "main.local"} {
		f := funcNamed(t, p, name)
		crossed := 0
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op != mir.OpUnboxDirect || p.R.Types.Kind(f.Locals[in.Args[0]].Type) != sem.KRef {
					continue
				}
				crossed++
				if p.CrossHandsOver(f, in.Args[0]) {
					t.Errorf("%s: the crossing out of %s hands over a count the reference does not hold:\n%s", name, f.Locals[in.Args[0]].Name, p.Dump())
				}
			}
		}
		if crossed == 0 {
			t.Errorf("%s: no crossing out of a reference:\n%s", name, p.Dump())
		}
	}
}
