package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/mir"
)

const specCompareSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn same[T: Eq](a: T, b: T) -> Bool { a == b }

fn main() -> Void! {
    let p = Point { x: 1, y: 2 }
    let q = Point { x: 1, y: 3 }
    let r = same(p, q)
}
`

func TestGenericComparisonKeepsItsNaNGuardOnlyInTheSharedBody(t *testing.T) {
	p := specialized(t, specCompareSrc)
	shared := funcNamed(t, p, "main.same")
	if _, ok := builtinNamed(shared, "cmp.unordered"); !ok {
		t.Fatalf("the shared body lost its NaN test:\n%s", p.Dump())
	}
	if _, ok := builtinNamed(shared, "cmp.guard"); !ok {
		t.Fatalf("the shared body lost its guard:\n%s", p.Dump())
	}
	seen := 0
	for _, f := range p.Funcs {
		if f == shared || !strings.HasPrefix(f.Name, "main.same") {
			continue
		}
		seen++
		if _, ok := builtinNamed(f, "cmp.unordered"); ok {
			t.Errorf("%s keeps the NaN test of a witness that is not a float:\n%s", f.Name, p.Dump())
		}
		if _, ok := builtinNamed(f, "cmp.guard"); ok {
			t.Errorf("%s keeps the guard of a folded NaN test:\n%s", f.Name, p.Dump())
		}
	}
	if seen == 0 {
		t.Fatalf("no instance of same was made:\n%s", p.Dump())
	}
	if err := p.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestGenericComparisonBuiltinsAreRegistered(t *testing.T) {
	for _, name := range []string{"cmp.unordered", "cmp.guard"} {
		if _, ok := mir.BuiltinFor(name); !ok {
			t.Errorf("no registry entry for %s", name)
		}
	}
}
