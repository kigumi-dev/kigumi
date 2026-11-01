package mir_test

import (
	"slices"
	"testing"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

const boxedParamSrc = `
fn put(r: &mut Int, i: Int) { r = i }

fn forward(r: &mut Int, i: Int, j: Int) { put(r, i) }

fn call(f: fn(Int) -> Bool, x: Int, y: Int) -> Bool { f(x) }

fn plain(i: Int, f: Float) -> Float { f * i.toFloat() }

fn mk(k: Int) -> fn(Int) -> Int { (x: Int) => x + k }

fn main() -> Void! {}
`

func TestBoxedParamsFollowTheBoxesABodyMakes(t *testing.T) {
	p := mirfixture.Planned(t, boxedParamSrc)
	bp := mir.NewBoxedParams(p)
	for name, want := range map[string][]bool{
		"main.put":     {false, true},
		"main.forward": {false, true, false},
		"main.call":    {false, true, false},
		"main.plain":   {false, false},
	} {
		if got := bp.Of(mirfixture.FuncNamed(t, p, name)); !slices.Equal(got, want) {
			t.Errorf("%s: boxed parameters %v, want %v", name, got, want)
		}
	}
}

func TestBoxedParamsTakeEveryClosureParameterBoxed(t *testing.T) {
	p := mirfixture.Planned(t, boxedParamSrc)
	bp := mir.NewBoxedParams(p)
	closures := 0
	for _, f := range p.Funcs {
		if f.Ent == 0 || p.R.Entity(f.Ent).Kind != sem.EntClosure {
			continue
		}
		closures++
		if !slices.Contains(bp.Of(f), true) {
			t.Errorf("%s: a closure takes its scalar parameters boxed, got %v", f.Name, bp.Of(f))
		}
	}
	if closures == 0 {
		t.Fatal("the source forms a closure")
	}
}

const mutualSrc = `
fn put(r: &mut Int, i: Int) { r = i }

fn f(r: &mut Int, i: Int, n: Int) {
    if n > 0 { g(r, i, n - 1) }
    put(r, i)
}

fn g(r: &mut Int, i: Int, n: Int) {
    if n > 0 { f(r, i, n - 1) }
}

fn main() -> Void! {}
`

func TestBoxedParamsOfMutualRecursionIgnoreTheOrderOfFirstReach(t *testing.T) {
	for _, first := range []string{"main.f", "main.g"} {
		p := mirfixture.Planned(t, mutualSrc)
		bp := mir.NewBoxedParams(p)
		bp.Of(mirfixture.FuncNamed(t, p, first))
		for _, name := range []string{"main.f", "main.g"} {
			if got, want := bp.Of(mirfixture.FuncNamed(t, p, name)), []bool{false, true, false}; !slices.Equal(got, want) {
				t.Errorf("%s first: %s has boxed parameters %v, want %v", first, name, got, want)
			}
		}
	}
}
