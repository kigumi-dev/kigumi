package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/sem"
)

const plannedCopySrc = `type Nm = {
    pub name String
    pub mut n Int
}

type W = {
    pub mut nm Nm
}

type Pt = {
    pub mut x Int
    pub mut y Int
}

type WP = {
    pub mut p Pt
    pub k Int
}

noalloc fn written(w: &W) -> Int {
    let mut b = w.nm
    b.n = 5
    b.n + w.nm.n
}

fn copies(w: &W) -> Int {
    let mut b = w.nm
    b.n = 5
    b.n
}

noalloc fn callsCopier(w: &W) -> Int {
    copies(w)
}

noalloc fn readOnly(w: &W) -> Int {
    let b = w.nm
    b.n
}

noalloc fn directCopy(w: &WP) -> Int {
    let mut b = w.p
    b.x = 5
    b.x + w.p.x
}

fn main() -> Void! {
    let w = W { nm: Nm { name: "a", n: 1 } }
    let wp = WP { p: Pt { x: 1, y: 2 }, k: 3 }
    print "${written(&w)} ${callsCopier(&w)} ${readOnly(&w)} ${directCopy(&wp)}"
}
`

func TestNoallocRejectsAPlannedCopy(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, plannedCopySrc)
	for _, want := range []string{
		"error[E1700]: copying `Nm` here allocates a box of its own for the copy\n  --> main.kg:21:13",
		"error[E907]: `copies` is not `noalloc`\n  --> main.kg:33:5",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "error["); n != 2 {
		t.Errorf("got %d errors, want 2:\n%s", n, out)
	}
}

func TestExplainNamesAPlannedCopy(t *testing.T) {
	t.Parallel()
	src := strings.ReplaceAll(plannedCopySrc, "noalloc fn", "fn")
	a := analyzeModule(t, src, false, 0)
	if a.Res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", a.Res.Render())
	}
	for name, want := range map[string]bool{"written": true, "copies": true, "callsCopier": true, "readOnly": false, "directCopy": false} {
		ex := a.Res.ExplainOpFact(mainFn(t, a.Res, name), sem.FactAlloc)
		if ex.Maybe != want {
			t.Errorf("%s may allocate = %v, want %v", name, ex.Maybe, want)
			continue
		}
		if want && !strings.Contains(ex.Chain[len(ex.Chain)-1].Text, "copies `Nm` into a box of its own") {
			t.Errorf("%s: chain %+v does not end at the copy", name, ex.Chain)
		}
	}
}

const lentReadSrc = `type Named = {
    pub name String
    pub mut n Int
}

type W = {
    pub mut nm Named
    pub mut other Named
    pub mut k Int
}

fn readsField(x: Named) -> Int {
    x.n
}

fn reader(w: &mut W) -> Int {
    w.k
}

noalloc fn viaRead(w: &W) -> Int {
    readsField(w.nm)
}

noalloc fn sibling(w: &mut W) -> Int {
    let b = w.nm
    w.k = 3
    w.other.n = 4
    b.n + w.k
}

noalloc fn viaReader(w: &mut W) -> Int {
    let b = w.nm
    reader(w) + b.n
}

fn main() -> Void! {
    let mut w = W { nm: Named { name: "a", n: 1 }, other: Named { name: "b", n: 2 }, k: 0 }
    print "${viaRead(&w)} ${sibling(&mut w)} ${viaReader(&mut w)}"
}
`

func TestNoallocKeepsAReadOfALentField(t *testing.T) {
	t.Parallel()
	if out := checkAllocClaim(t, lentReadSrc); strings.Contains(out, "error[") {
		t.Fatalf("unexpected errors:\n%s", out)
	}
	a := analyzeModule(t, strings.ReplaceAll(lentReadSrc, "noalloc fn", "fn"), false, 0)
	for _, name := range []string{"viaRead", "sibling", "viaReader"} {
		if ex := a.Res.ExplainOpFact(mainFn(t, a.Res, name), sem.FactAlloc); ex.Maybe {
			t.Errorf("%s may allocate: %+v", name, ex.Chain)
		}
	}
}

func mainFn(t *testing.T, r *sem.Result, name string) sem.EntityID {
	t.Helper()
	for id := 1; id < len(r.Entities); id++ {
		e := r.Entity(sem.EntityID(id))
		if e.Kind == sem.EntFn && e.Name == name && !r.Packages[e.Pkg].Std {
			return sem.EntityID(id)
		}
	}
	t.Fatalf("no function %s", name)
	return 0
}
