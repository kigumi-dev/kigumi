package driver_test

import (
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/sem"
)

const genericCopySrc = `type Named = {
    pub name String
    pub mut n Int
}

fn incr(v: &mut Int) {
    v = v + 1
}

fn bump(v: &mut Named) {
    v.n = v.n + 1
}

fn id[T: Copy](x: T, f: fn(&mut T) -> Void) -> T {
    let mut y = x
    f(&mut y)
    x
}

fn keep[T: Copy](x: T, f: fn(&mut T) -> Void) -> T {
    x
}

fn main() -> Void! {
    let named = Named { name: "a", n: 1 }
    let mut total = 0
    for i in 0..6 {
        total = total + %s
    }
    print "${total}"
}
`

func genericCopyAllocs(t *testing.T, call string) int {
	t.Helper()
	return nativeAllocs(t, fmt.Sprintf(genericCopySrc, call))
}

func TestGenericCopyAllocatesOnlyForABoxedArgument(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	if copied, kept := genericCopyAllocs(t, "id(i, incr)"), genericCopyAllocs(t, "keep(i, incr)"); copied != kept {
		t.Errorf("copying a scalar T allocates: %d allocations against %d", copied, kept)
	}
	if copied, kept := genericCopyAllocs(t, "id(named, bump).n"), genericCopyAllocs(t, "keep(named, bump).n"); copied <= kept {
		t.Errorf("copying a boxed T allocates nothing: %d allocations against %d", copied, kept)
	}
}

const genericDecidedSrc = `type Named = {
    pub name String
    pub mut n Int
}

type Pt = {
    pub x Int
    pub y Int
}

noalloc fn id[T: Copy](x: T) -> T {
    let y = x
    y
}

noalloc fn viaId[T: Copy](x: T) -> T {
    id(x)
}

noalloc fn onInt(n: Int) -> Int {
    id(n)
}

noalloc fn onStr(s: String) -> String {
    id(s)
}

noalloc fn onPt(p: Pt) -> Pt {
    id(p)
}

noalloc fn onNested(n: Int) -> Int {
    viaId(n)
}

noalloc fn onNamed(r: Named) -> Named {
    id(r)
}

noalloc fn onNestedNamed(r: Named) -> Named {
    viaId(r)
}

fn main() -> Void! {
    print "${onInt(1)}"
}
`

func TestGenericNoallocCopyIsDecidedAtTheCall(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, genericDecidedSrc)
	for _, want := range []string{"--> main.kg:37:5", "--> main.kg:41:5"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing an E1700 at %s in:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "error["); n != 2 {
		t.Errorf("got %d errors, want the two calls with a boxed argument:\n%s", n, out)
	}
	if !strings.Contains(out, "copying `Named` here allocates a box of its own for the copy") {
		t.Errorf("the call does not name the copied type:\n%s", out)
	}
}

const explainInstSrc = `type Named = {
    pub name String
    pub mut n Int
}

noalloc fn id[T: Copy](x: T) -> T {
    let y = x
    y
}

fn hand(r: Named) -> Named {
    id(r)
}

fn first(a: &Array[String]) -> Int {
    if a.get(0) is Some(_) { 1 } else { 0 }
}

fn main() -> Void! {
    print "${hand(Named { name: "a", n: 1 }).n} ${first(&Array.of("b"))}"
}
`

func TestExplainNamesACallThatCopiesATypeArgument(t *testing.T) {
	t.Parallel()
	a := analyzeModule(t, explainInstSrc, false, 0)
	if a.Res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", a.Res.Render())
	}
	ex := a.Res.ExplainOpFact(mainFn(t, a.Res, "hand"), sem.FactAlloc)
	if !ex.Maybe {
		t.Fatalf("hand does not report an allocation")
	}
	if last := ex.Chain[len(ex.Chain)-1].Text; !strings.Contains(last, "copies `Named` into a box of its own here, because `id` copies its type argument") {
		t.Errorf("chain %+v does not end at the instantiated copy", ex.Chain)
	}
	ex = a.Res.ExplainOpFact(mainFn(t, a.Res, "first"), sem.FactAlloc)
	if !ex.Maybe {
		t.Fatalf("first does not report an allocation")
	}
	if last := ex.Chain[len(ex.Chain)-1].Text; !strings.Contains(last, "boxes `String?` for the result here") {
		t.Errorf("chain %+v does not end at the boxed element", ex.Chain)
	}
}
