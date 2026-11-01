package plan_test

import (
	"testing"

	"kigumi/internal/devkit/mirfixture"
)

const pathSrc = `
type Named = {
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

fn keepsN(x: Named, out: &mut Array[Int]) {
    out.push(x.n)
}

fn setN(mut x: Named) -> Int {
    x.n = 5
    x.n
}

fn reader(w: &mut W) -> Int {
    w.k + w.nm.n
}

fn deepReader(w: &mut W) -> Int {
    reader(w) + 1
}

fn writer(w: &mut W) {
    w.nm.n = 4
}

fn deepWriter(w: &mut W) {
    writer(w)
}

fn peek(x: &mut Named) -> Int {
    x.n
}

fn poke(x: &mut Named) {
    x.n = 7
}

fn viaRead(w: &W) -> Int {
    readsField(w.nm)
}

fn viaKeep(w: &W, out: &mut Array[Int]) {
    keepsN(w.nm, out)
}

fn viaSet(w: &W) -> Int {
    setN(w.nm)
}

fn sibling(w: &mut W) -> Int {
    let b = w.nm
    w.k = 3
    w.other.n = 4
    b.n + w.k
}

fn sameField(w: &mut W) -> Int {
    let b = w.nm
    w.nm.n = 4
    b.n
}

fn siblingPoke(w: &mut W) -> Int {
    let b = w.nm
    poke(&mut w.other)
    b.n
}

fn samePoke(w: &mut W) -> Int {
    let b = w.nm
    poke(&mut w.nm)
    b.n
}

fn viaReader(w: &mut W) -> Int {
    let b = w.nm
    deepReader(w) + peek(&mut w.nm) + b.n
}

fn viaWriter(w: &mut W) -> Int {
    let b = w.nm
    deepWriter(w)
    b.n
}

fn rebound(mut w: W) -> Int {
    let b = w.nm
    w = W { nm: Named { name: "r", n: 2 }, other: Named { name: "s", n: 3 }, k: 0 }
    w.other.n = 9
    b.n
}

fn main() -> Void! {}
`

func TestPlanRCLendsAFieldToAReader(t *testing.T) {
	prog := rcPlanned(t, pathSrc)
	for name, want := range map[string]int{"main.viaRead": 0, "main.viaKeep": 0, "main.viaSet": 1} {
		if got := len(copies(mirfixture.FuncNamed(t, prog, name))); got != want {
			t.Fatalf("%s: %d deep copies, want %d\n%s", name, got, want, prog.Dump())
		}
	}
}

func TestPlanRCCopiesOnlyWhatAWriteReaches(t *testing.T) {
	prog := rcPlanned(t, pathSrc)
	for name, want := range map[string]int{
		"main.sibling": 0, "main.sameField": 1, "main.siblingPoke": 0, "main.samePoke": 1,
		"main.viaReader": 0, "main.viaWriter": 1, "main.rebound": 1,
	} {
		if got := len(copies(mirfixture.FuncNamed(t, prog, name))); got != want {
			t.Fatalf("%s: %d deep copies, want %d\n%s", name, got, want, prog.Dump())
		}
	}
}
