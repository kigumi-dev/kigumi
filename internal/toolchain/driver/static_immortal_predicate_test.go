package driver_test

import (
	"testing"

	"kigumi/internal/toolchain/driver"
)

const immortalStaticPredicateSrc = `type Q = { x Int, s String, ok Bool, f Float, c Char }
type In = { a Int }
type Out = { p In, q Int }
type Pt = { mut x Int }
type Holds = { p Pt, q Int }
type Buf = { b Bytes, n Int }

fn mkQ() -> Q { Q { x: 1, s: "s", ok: true, f: 1.5, c: 'c' } }
fn mkOut() -> Out { Out { p: In { a: 1 }, q: 2 } }
fn mkPt() -> Pt { Pt { x: 1 } }
fn mkHolds() -> Holds { Holds { p: Pt { x: 1 }, q: 2 } }
fn mkBuf() -> Buf { Buf { b: b"x", n: 1 } }

static flat: Q = comptime mkQ()
static nested: Out = comptime mkOut()
static mutField: Pt = comptime mkPt()
static nestedMut: Holds = comptime mkHolds()
static mut counter: Out = comptime mkOut()
static scalar: Int = comptime { 3 }
static buf: Buf = comptime mkBuf()

fn main() {
    print "ok"
}
`

func TestImmortalStaticPredicate(t *testing.T) {
	t.Parallel()
	a, err := driver.Analyze(loadSweepModule(t, immortalStaticPredicateSrc, ""))
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatal(a.Res.Render())
	}
	want := map[string]bool{"flat": true, "nested": true, "mutField": false, "nestedMut": false, "counter": false, "scalar": false, "buf": false}
	for _, ent := range a.Res.StaticEntities() {
		name := a.Res.Entity(ent).Name
		if w, ok := want[name]; ok {
			if got := a.Res.ImmortalStatic(ent); got != w {
				t.Errorf("ImmortalStatic(%s) = %v, want %v", name, got, w)
			}
			delete(want, name)
		}
	}
	if len(want) != 0 {
		t.Errorf("statics not found: %v", want)
	}
}
