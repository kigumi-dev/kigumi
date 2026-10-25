package llgen_test

import (
	"regexp"
	"slices"
	"testing"

	"kigumi/internal/llgen"
	"kigumi/internal/mir"
)

const argBoxSrc = `
import map from std/map

fn same[T: Eq](a: T, b: &T) -> Bool {
    (&a).equals(b)
}

fn twice(x: Int) -> Int {
    x * 2
}

fn fill(m: &mut map.Map[Int, Int], xs: &mut Array[Int], i: Int, f: Float, c: Bool) -> Int {
    _ = m.trySet(i * 100003, i + 1)
    _ = m.trySet(7, 9)
    _ = same(f * 2.0, &1.5)
    let k = if c { 5000 } else { 6000 }
    _ = xs.tryPush(k)
    let mut j = 7000
    if c {
        j = 8000
    }
    _ = xs.tryInsertAt(0, j)
    let mut v = 9000
    let mut n = 0
    for n < 2 {
        _ = xs.tryPush(v)
        v = 9500
        n = n + 1
    }
    twice(i * 3)
}

fn main() {
    let mut m = map.empty[Int, Int]()
    let mut xs = Array.empty[Int]()
    let _ = fill(&mut m, &mut xs, 3, 1.5, true)
}
`

var scalarBoxCall = regexp.MustCompile(`call ptr @rt_(int|float|char|box_int128|binop|unop|cast)\(`)

func paramOf(p *mir.Program, ent mir.Inst, i int) string {
	if g := p.ByEnt[ent.Ent]; g != nil {
		return g.Locals[g.Params[i]].Name
	}
	return p.R.Entity(p.R.Fn(ent.Ent).Params[i-1]).Name
}

// The hook lists exactly the scalar boxes fill's emitted body makes for call arguments: computed values, a phi of
// constants, a reassigned local and a loop-carried one, into bodies and a bodiless primitive alike.
func TestScalarArgBoxesMatchTheEmittedBoxes(t *testing.T) {
	p := plannedProgram(t, argBoxSrc)
	opts := llgen.EmitOptions{Reachable: mir.Reachable(p)}
	var fill *mir.Func
	for _, f := range p.Funcs {
		if f.Name == "main.fill" {
			fill = f
		}
	}
	if fill == nil {
		t.Fatal("no main.fill")
	}
	var got []string
	for _, ab := range llgen.ScalarArgBoxes(p, opts)[fill] {
		got = append(got, paramOf(p, fill.Blocks[ab.Block].Insts[ab.Inst], ab.Arg))
	}
	slices.Sort(got)
	if want := []string{"a", "item", "item", "item", "key", "value"}; !slices.Equal(got, want) {
		t.Errorf("boxed parameters %v, want %v", got, want)
	}
	body := fnBody(t, llgen.EmitProgram(p, opts).IR, "main.fill")
	if n := len(scalarBoxCall.FindAllString(body, -1)); n != len(got) {
		t.Errorf("the emitted body makes %d scalar boxes, the hook lists %d:\n%s", n, len(got), body)
	}
}
