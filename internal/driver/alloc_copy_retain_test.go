package driver_test

import (
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/sem"
)

const retainTreeSrc = `type Tree = Leaf(v: Int) | Node(kids: FixedArray[Tree, 2])

fn sum(t: Tree) -> Int {
    match t {
        Leaf(v) -> v
        Node(kids) -> {
            let mut s = 0
            for k in kids {
                s = s + sum(k)
            }
            s
        }
    }
}

fn peek(t: Tree) -> Int {
    match t {
        Leaf(v) -> v
        Node(_) -> 1
    }
}

fn build(depth: Int, base: Int) -> Tree {
    if depth == 0 {
        return Leaf(base)
    }
    let mut kids = FixedArray.filled[Tree, 2](Leaf(0))
    kids[0] = build(depth - 1, base * 2)
    kids[1] = build(depth - 1, base * 2 + 1)
    Node(kids)
}

fn main() -> Unit! {
    let t = build(3, 1)
    let mut total = 0
    for i in 0..6 {
        total = total + %s(t)
    }
    print "${total}"
}
`

func TestRecursiveSumRegainsNoalloc(t *testing.T) {
	t.Parallel()
	a := analyzeModule(t, fmt.Sprintf(retainTreeSrc, "sum"), false, 0)
	if a.Res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", a.Res.Render())
	}
	if !a.Res.Summary(mainFn(t, a.Res, "sum")).Noalloc {
		t.Errorf("sum is not inferred noalloc")
	}
}

const genericCopySrc = `type Named = {
    pub name String
    pub mut n Int
}

fn id[T: Copy](x: T) -> T {
    let y = x
    y
}

fn keep[T: Copy](x: T) -> T {
    x
}

fn main() -> Unit! {
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
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	if copied, kept := genericCopyAllocs(t, "id(i)"), genericCopyAllocs(t, "keep(i)"); copied != kept {
		t.Errorf("copying a scalar T allocates: %d allocations against %d", copied, kept)
	}
	if copied, kept := genericCopyAllocs(t, "id(named).n"), genericCopyAllocs(t, "keep(named).n"); copied <= kept {
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

fn main() -> Unit! {
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

const noallocArrayGetSrc = `type Named = {
    pub name String
    pub mut n Int
}

noalloc fn getInt(a: &Array[Int], i: usize) -> Int {
    let v = a.get(i)
    if v is None { 0 } else { 1 }
}

noalloc fn getStr(a: &Array[String], i: usize) -> Int {
    let v = a.get(i)
    if v is None { 0 } else { 1 }
}

noalloc fn getNamed(a: &Array[Named], i: usize) -> Int {
    let v = a.get(i)
    if v is None { 0 } else { 1 }
}

noalloc fn getAny[T: Copy](a: &Array[T], i: usize) -> Int {
    let v = a.get(i)
    if v is None { 0 } else { 1 }
}

fn main() -> Unit! {
    print "${1}"
}
`

func TestNoallocArrayGetIsChargedUnlessDirect(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, noallocArrayGetSrc)
	for _, want := range []string{"--> main.kg:12:13", "--> main.kg:17:13", "--> main.kg:22:13"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing an E1700 at %s in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "main.kg:7:") {
		t.Errorf("a scalar element is read directly and must stay noalloc:\n%s", out)
	}
	if n := strings.Count(out, "error["); n != 3 {
		t.Errorf("got %d errors, want 3:\n%s", n, out)
	}
}

func TestNoallocFirstIsNoLongerPromised(t *testing.T) {
	t.Parallel()
	src := "noalloc fn head(a: &Array[Int]) -> Int {\n    a.first() || 0\n}\n\nfn main() -> Unit! {\n    print \"${head(&Array.of(1))}\"\n}\n"
	if out := checkAllocClaim(t, src); !strings.Contains(out, "`first` is not `noalloc`") {
		t.Errorf("first still claims noalloc:\n%s", out)
	}
}

const getLoopSrc = `fn main() -> Unit! {
    let a = Array.of(%s)
    let mut s = 0
    for i in 0..10 {
        %s
    }
    print "${s}"
}
`

func TestArrayGetBoxesItsOptionUnlessDirect(t *testing.T) {
	t.Parallel()
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	count := func(elems, body string) int { return nativeAllocs(t, fmt.Sprintf(getLoopSrc, elems, body)) }
	if direct, none := count("1, 2", "if a.get(1) is Some(_) { s = s + 1 }"), count("1, 2", "s = s + 1"); direct != none {
		t.Errorf("reading a scalar element allocates: %d allocations against %d", direct, none)
	}
	if boxed, none := count(`"x", "y"`, "if a.get(1) is Some(_) { s = s + 1 }"), count(`"x", "y"`, "s = s + 1"); boxed <= none {
		t.Errorf("reading a String element allocates nothing: %d allocations against %d", boxed, none)
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

fn main() -> Unit! {
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
