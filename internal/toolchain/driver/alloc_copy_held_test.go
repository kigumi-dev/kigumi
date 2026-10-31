package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
)

func TestNoallocRejectsAHeldCopy(t *testing.T) {
	t.Parallel()
	src := "noalloc fn sameStart(a: Bytes, b: Bytes) -> Bool {\n    a.startsWith(b)\n}\n\nfn main() -> Void! {\n    print \"${sameStart(\"abc\".toBytes(), \"ab\".toBytes())}\"\n}\n"
	out := checkAllocClaim(t, src)
	want := "error[E1700]: copying `Bytes` here allocates a box of its own for the copy\n --> main.kg:2:5"
	if !strings.Contains(out, want) {
		t.Errorf("missing %q in:\n%s", want, out)
	}
	if n := strings.Count(out, "error["); n != 1 {
		t.Errorf("got %d errors, want 1:\n%s", n, out)
	}
}

func TestExplainNamesAHeldCopy(t *testing.T) {
	t.Parallel()
	src := "fn sameStart(a: Bytes, b: Bytes) -> Bool {\n    a.startsWith(b)\n}\n\nfn main() -> Void! {\n    print \"${sameStart(\"abc\".toBytes(), \"ab\".toBytes())}\"\n}\n"
	a := analyzeModule(t, src, false, 0)
	if a.Res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", a.Res.Render())
	}
	ex := a.Res.ExplainOpFact(mainFn(t, a.Res, "sameStart"), sem.FactAlloc)
	if !ex.Maybe {
		t.Fatalf("sameStart does not report an allocation")
	}
	if last := ex.Chain[len(ex.Chain)-1].Text; !strings.Contains(last, "copies `Bytes` into a box of its own here, because the local keeps its own value") {
		t.Errorf("chain %+v does not end at the held copy", ex.Chain)
	}
}

func TestStdBytesPrefixHelpersCopyNothing(t *testing.T) {
	t.Parallel()
	src := "fn main() -> Void! {\n    let b = \"abc\".toBytes()\n    print \"${b.startsWith(\"ab\".toBytes())} ${b.endsWith(\"bc\".toBytes())}\"\n}\n"
	if out := checkAllocClaim(t, src); strings.Contains(out, "error[") {
		t.Fatalf("unexpected errors:\n%s", out)
	}
}

const swapSrc = `type Nm = {
    pub name String
    pub mut n Int
}

type V = {
    pub mut a Nm
    pub mut c Nm
}

noalloc fn swap(w: &mut V) {
    let t = w.a
    w.a = w.c
    w.c = t
}

fn mk() -> V {
    V { a: Nm { name: "a", n: 1 }, c: Nm { name: "c", n: 2 } }
}

fn run() -> Int {
    let mut w = mk()
    %s
    w.a.n = 5
    w.a.n + w.c.n
}

print "ok=${run()}"
`

func TestSwapStoresMoveTheirValue(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	swapped := nativeAllocs(t, strings.Replace(swapSrc, "%s", "swap(&mut w)\n    swap(&mut w)", 1))
	plain := nativeAllocs(t, strings.Replace(swapSrc, "%s", "", 1))
	if swapped != plain {
		t.Errorf("two swaps allocate: %d allocations, %d without them", swapped, plain)
	}
}

const arrayGetSrc = `type Nm = {
    pub name String
    pub mut n Int
}

noalloc fn nth(arr: &Array[Nm], i: usize) -> Int {
    match arr.get(i) {
        Some(k) -> k.n
        None -> 0
    }
}

noalloc fn nthInt(arr: &Array[Int], i: usize) -> Int {
    match arr.get(i) {
        Some(k) -> k
        None -> 0
    }
}

fn main() -> Void! {
    let a = Array.of(Nm { name: "a", n: 1 })
    print "${nth(&a, 0)} ${nthInt(&Array.of(1), 0)}"
}
`

func TestNoallocRejectsAnElementCopy(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, arrayGetSrc)
	want := "error[E1700]: copying `Nm` here allocates a box of its own for the copy\n --> main.kg:7:11"
	if !strings.Contains(out, want) {
		t.Errorf("missing %q in:\n%s", want, out)
	}
	if n := strings.Count(out, "error["); n != 1 {
		t.Errorf("got %d errors, want 1 (the scalar element stays noalloc):\n%s", n, out)
	}
}
