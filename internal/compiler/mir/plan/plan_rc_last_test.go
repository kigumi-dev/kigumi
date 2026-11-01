package plan_test

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/devkit/mirfixture"
)

const lastSrc = `
type Named = {
    pub name String
    pub mut n Int
}

type W = {
    pub mut nm Named
    pub mut other Named
    pub k Int
}

type Holder[F] = {
    pub mut f F
}

fn swap(w: &mut W) {
    let t = w.nm
    w.nm = w.other
    w.other = t
}

fn keep(w: &mut W) {
    let t = w.nm
    w.other = t
    w.nm.n = 4
}

fn retLent(w: &W) -> Named {
    let t = w.nm
    t
}

fn pass(n: Named) -> Named {
    n
}

fn run[F: FnMut[fn() -> Void]](x: Named, mut f: F) -> Int {
    f()
    x.n
}

fn reads(x: Named, w: &W) -> Int {
    x.n + w.k
}

fn writes(x: Named, w: &mut W) -> Int {
    w.nm.n = 9
    x.n
}

fn runHeld[F: FnMut[fn() -> Void]](x: Named, mut h: Holder[F]) -> Int {
    h.f()
    x.n
}

fn mkw() -> W {
    W { nm: Named { name: "a", n: 1 }, other: Named { name: "b", n: 2 }, k: 0 }
}

fn viaNested() -> Int {
    let mut w = mkw()
    let inner = () => { w.nm.n = 31 }
    let outer = () => { inner() }
    run(w.nm, outer)
}

fn viaHolder() -> Int {
    let mut w = mkw()
    let h = Holder { f: () => { w.nm.n = 21 } }
    runHeld(w.nm, h)
}

fn viaOtherRoot() -> Int {
    let w = mkw()
    let mut v = mkw()
    let h = Holder { f: () => { v.nm.n = 21 } }
    runHeld(w.nm, h)
}

fn viaClosure(w: &mut W) -> Int {
    run(w.nm, () => { w.nm.n = 9 })
}

fn viaShared[F: FnMut[fn() -> Void]](w: &W, mut f: F) -> Int {
    run(w.nm, f)
}

fn viaMut[F: FnMut[fn() -> Void]](w: &mut W, mut f: F) -> Int {
    run(w.nm, f)
}

fn viaOwned[F: FnMut[fn() -> Void]](w: W, mut f: F) -> Int {
    run(w.nm, f)
}

fn viaReader(w: &W) -> Int {
    reads(w.nm, w)
}

fn viaWriter(w: &mut W) -> Int {
    writes(w.nm, w)
}

fn main() -> Void! {}
`

func sharesOf(f *midir.Func, op midir.Opcode, own bool) int {
	n := 0
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == op && (op != midir.OpShare || in.Copy && in.Own == own) {
				n++
			}
		}
	}
	return n
}

func TestPlanDirectMovesAValueLeftBehind(t *testing.T) {
	p := mirfixture.Planned(t, lastSrc)
	swap := mirfixture.FuncNamed(t, p, "main.swap")
	if n := sharesOf(swap, midir.OpShare, false) + sharesOf(swap, midir.OpShare, true); n != 0 {
		t.Errorf("swap copies %d times:\n%s", n, p.Dump())
	}
	if n := sharesOf(swap, midir.OpMove, false); n != 1 {
		t.Errorf("swap moves %d times, want 1:\n%s", n, p.Dump())
	}
	for _, name := range []string{"main.keep", "main.retLent", "main.pass"} {
		f := mirfixture.FuncNamed(t, p, name)
		if sharesOf(f, midir.OpShare, true)+sharesOf(f, midir.OpShare, false) == 0 {
			t.Errorf("%s hands over a box something else still holds:\n%s", name, p.Dump())
		}
	}
}

func TestPlanRCCopiesWhatACallWrites(t *testing.T) {
	p := mirfixture.Planned(t, lastSrc)
	for name, want := range map[string]int{"main.viaClosure": 1, "main.viaNested": 1, "main.viaHolder": 1, "main.viaOtherRoot": 0, "main.viaShared": 1, "main.viaMut": 1, "main.viaOwned": 0, "main.viaReader": 0, "main.viaWriter": 1} {
		if got := sharesOf(mirfixture.FuncNamed(t, p, name), midir.OpShare, true); got != want {
			t.Errorf("%s: %d planned copies, want %d:\n%s", name, got, want, p.Dump())
		}
	}
}
