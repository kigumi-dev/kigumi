package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

const copyStorePlaceSrc = `
type Pt = {
    pub mut x Int
    pub mut y Int
}

type Wrap = {
    pub mut a FixedArray[Pt, 3]
}

noalloc fn viaField(w: &mut Wrap) -> Unit {
    w.a[0] = w.a[1]
}

fn Wrap.shift(mut self) -> Unit {
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

fn main() -> Unit! {}
`

func TestCopyStoreThroughAPlaceFillsTheSlotInPlace(t *testing.T) {
	p := planned(t, copyStorePlaceSrc)
	for _, name := range []string{"main.viaField", "main.Wrap.shift"} {
		f := funcNamed(t, p, name)
		if got := countOp(f, mir.OpBoxDirect); got != 0 {
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
		if got := len(p.CopyStores(f)); got != 1 || countOp(f, mir.OpBoxDirect) != 0 {
			t.Errorf("%s: %d in-place copy stores, %d boxings:\n%s", f.Name, got, countOp(f, mir.OpBoxDirect), p.Dump())
		}
	}
	if !found {
		t.Error("no closure planned")
	}
}
