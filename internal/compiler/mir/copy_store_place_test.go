package mir_test

import (
	"strings"
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

const copyStorePlaceSrc = `
type Pt = {
    pub mut x Int
    pub mut y Int
}

type Wrap = {
    pub mut a FixedArray[Pt, 3]
}

noalloc fn viaField(w: &mut Wrap) {
    w.a[0] = w.a[1]
}

fn Wrap.shift(mut self) {
    self.a[0] = self.a[2]
}

fn closure() -> Int {
    let mut cs = FixedArray.filled[Pt, 3](Pt { x: 1, y: 2 })
    let f = () => {
        cs[0] = cs[1]
    }
    f()
    cs[0].x
}

fn main() -> Void! {}
`

func TestCopyStoreThroughAPlaceFillsTheSlotInPlace(t *testing.T) {
	p := mirfixture.Planned(t, copyStorePlaceSrc)
	for _, name := range []string{"main.viaField", "main.Wrap.shift"} {
		f := mirfixture.FuncNamed(t, p, name)
		if got := mirfixture.CountOp(f, midir.OpBoxDirect); got != 0 {
			t.Errorf("%s boxes %d times:\n%s", name, got, p.Dump())
		}
		if got := len(p.CopyStores(f)); got != 1 {
			t.Errorf("%s has %d in-place copy stores, want 1:\n%s", name, got, p.Dump())
		}
	}
	found := false
	for _, f := range p.Funcs {
		if f.Ent == 0 || p.R.Entity(f.Ent).Kind != sem.EntClosure || !strings.HasPrefix(f.Name, "main.") {
			continue
		}
		found = true
		if got := len(p.CopyStores(f)); got != 1 || mirfixture.CountOp(f, midir.OpBoxDirect) != 0 {
			t.Errorf("%s: %d in-place copy stores, %d boxings:\n%s", f.Name, got, mirfixture.CountOp(f, midir.OpBoxDirect), p.Dump())
		}
	}
	if !found {
		t.Error("no closure planned")
	}
}

const copyStoreFieldSrc = `
type Pt = {
    pub mut x Int
    pub mut y Int
}

type Seg = {
    pub mut a Pt
    pub mut b Pt
}

type Wrap = {
    pub mut a FixedArray[Pt, 3]
}

type Outer = {
    pub mut segs FixedArray[Seg, 2]
    pub mut w Wrap
}

noalloc fn fieldFromField(o: &mut Outer) {
    o.segs[0].a = o.segs[1].b
}

noalloc fn slotFromField(o: &mut Outer) {
    o.w.a[1] = o.segs[1].a
}

fn Outer.shift(mut self) {
    self.segs[1].a = self.segs[0].b
}

fn byValueFields(mut o: Outer) {
    o.segs[0].a = o.segs[1].b
}

fn byValueSlots(mut w: Wrap) {
    w.a[0] = w.a[2]
}

fn main() -> Void! {}
`

func TestCopyStoreBetweenFieldsFillsTheDestinationInPlace(t *testing.T) {
	p := mirfixture.Planned(t, copyStoreFieldSrc)
	for _, name := range []string{"main.fieldFromField", "main.slotFromField", "main.Outer.shift", "main.byValueFields", "main.byValueSlots"} {
		f := mirfixture.FuncNamed(t, p, name)
		if got := mirfixture.CountOp(f, midir.OpBoxDirect); got != 0 {
			t.Errorf("%s boxes %d times:\n%s", name, got, p.Dump())
		}
		if got := len(p.CopyStores(f)); got != 1 {
			t.Errorf("%s has %d in-place copy stores, want 1:\n%s", name, got, p.Dump())
		}
	}
}
