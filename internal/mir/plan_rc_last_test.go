package mir_test

import (
	"testing"

	"kigumi/internal/mir"
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

fn swap(w: &mut W) -> Unit {
    let t = w.nm
    w.nm = w.other
    w.other = t
}

fn keep(w: &mut W) -> Unit {
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

fn run[F: FnMut[fn() -> Unit]](x: Named, mut f: F) -> Int {
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

fn runHeld[F: FnMut[fn() -> Unit]](x: Named, mut h: Holder[F]) -> Int {
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

fn viaShared[F: FnMut[fn() -> Unit]](w: &W, mut f: F) -> Int {
    run(w.nm, f)
}

fn viaMut[F: FnMut[fn() -> Unit]](w: &mut W, mut f: F) -> Int {
    run(w.nm, f)
}

fn viaOwned[F: FnMut[fn() -> Unit]](w: W, mut f: F) -> Int {
    run(w.nm, f)
}

fn viaReader(w: &W) -> Int {
    reads(w.nm, w)
}

fn viaWriter(w: &mut W) -> Int {
    writes(w.nm, w)
}

fn main() -> Unit! {}
`

func sharesOf(f *mir.Func, op mir.Opcode, own bool) int {
	n := 0
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == op && (op != mir.OpShare || in.Copy && in.Own == own) {
				n++
			}
		}
	}
	return n
}

func TestPlanDirectMovesAValueLeftBehind(t *testing.T) {
	p := planned(t, lastSrc)
	swap := funcNamed(t, p, "main.swap")
	if n := sharesOf(swap, mir.OpShare, false) + sharesOf(swap, mir.OpShare, true); n != 0 {
		t.Errorf("swap copies %d times:\n%s", n, p.Dump())
	}
	if n := sharesOf(swap, mir.OpMove, false); n != 1 {
		t.Errorf("swap moves %d times, want 1:\n%s", n, p.Dump())
	}
	for _, name := range []string{"main.keep", "main.retLent", "main.pass"} {
		f := funcNamed(t, p, name)
		if sharesOf(f, mir.OpShare, true)+sharesOf(f, mir.OpShare, false) == 0 {
			t.Errorf("%s hands over a box something else still holds:\n%s", name, p.Dump())
		}
	}
}

func TestPlanRCCopiesWhatACallWrites(t *testing.T) {
	p := planned(t, lastSrc)
	for name, want := range map[string]int{"main.viaClosure": 1, "main.viaNested": 1, "main.viaHolder": 1, "main.viaOtherRoot": 0, "main.viaShared": 1, "main.viaMut": 1, "main.viaOwned": 0, "main.viaReader": 0, "main.viaWriter": 1} {
		if got := sharesOf(funcNamed(t, p, name), mir.OpShare, true); got != want {
			t.Errorf("%s: %d planned copies, want %d:\n%s", name, got, want, p.Dump())
		}
	}
}
