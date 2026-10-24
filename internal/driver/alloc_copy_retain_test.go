package driver_test

import (
	"fmt"
	"strings"
	"testing"
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

fn main() -> Void! {
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

noalloc fn anyInt(a: &Array[Int], i: usize) -> Int {
    getAny(a, i)
}

noalloc fn anyNamed(a: &Array[Named], i: usize) -> Int {
    getAny(a, i)
}

fn main() -> Void! {
    print "${1}"
}
`

func TestNoallocArrayGetIsChargedUnlessDirect(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, noallocArrayGetSrc)
	for _, want := range []string{"--> main.kg:12:13", "--> main.kg:17:13", "--> main.kg:31:5"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing an E1700 at %s in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "main.kg:7:") || strings.Contains(out, "main.kg:27:") {
		t.Errorf("a scalar element is read directly and must stay noalloc:\n%s", out)
	}
	if n := strings.Count(out, "error["); n != 3 {
		t.Errorf("got %d errors, want 3:\n%s", n, out)
	}
}

func TestNoallocFirstIsChargedLikeGet(t *testing.T) {
	t.Parallel()
	scalar := "noalloc fn head(a: &Array[Int]) -> Int {\n    a.first() || 0\n}\n\nfn main() -> Void! {\n    print \"${head(&Array.of(1))}\"\n}\n"
	if out := checkAllocClaim(t, scalar); strings.Contains(out, "error[") {
		t.Errorf("a scalar first must stay noalloc:\n%s", out)
	}
	heap := "noalloc fn head(a: &Array[String]) -> Bool {\n    a.first() is Some(_)\n}\n\nfn main() -> Void! {\n    print \"${head(&Array.of(\"a\"))}\"\n}\n"
	if out := checkAllocClaim(t, heap); !strings.Contains(out, "error[E1700]") {
		t.Errorf("a heap element copied by first must be an E1700:\n%s", out)
	}
}

const getLoopSrc = `fn main() -> Void! {
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
	if testCompiler().CCompiler() == nil {
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
