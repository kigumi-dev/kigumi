package driver_test

import (
	"testing"

	"kigumi/internal/toolchain/driver"
)

const copyStoreHead = `type Pt = { pub mut x Int, pub mut y Int }
type Wrap = { pub mut a FixedArray[Pt, 3] }

fn mkWrap() -> Wrap {
    let mut w = Wrap { a: FixedArray.filled[Pt, 3](Pt { x: 1, y: 2 }) }
    w.a[1] = Pt { x: 5, y: 6 }
    w
}

`

var copyStoreSources = map[string]string{
	"local": `fn main() {
    let mut w = mkWrap()
    eprint "go"
    w.a[0] = w.a[1]
    if w.a[0].x + w.a[1].x != 10 { let none: Int? = None; none.unwrap() }
    print "x=${w.a[0].x}"
}
`,
	"local_held": `fn main() {
    let mut w = mkWrap()
    let held = w.a[0]
    eprint "go"
    w.a[0] = w.a[1]
    if held.x + w.a[0].x != 6 { let none: Int? = None; none.unwrap() }
    print "x=${held.x}"
}
`,
	"reference": `fn shift(w: &mut Wrap) {
    w.a[0] = w.a[1]
}

fn main() {
    let mut w = mkWrap()
    eprint "go"
    shift(&mut w)
    if w.a[0].x + w.a[1].x != 10 { let none: Int? = None; none.unwrap() }
    print "x=${w.a[0].x}"
}
`,
	"mut_self_held": `fn Wrap.shift(mut self) -> Int {
    let held = self.a[0]
    self.a[0] = self.a[1]
    held.x
}

fn main() {
    let mut w = mkWrap()
    eprint "go"
    let old = w.shift()
    if old + w.a[0].x != 6 || w.a[1].x != 5 { let none: Int? = None; none.unwrap() }
    print "x=${old}"
}
`,
	"closure_held": `fn main() {
    let mut cs = FixedArray.filled[Pt, 3](Pt { x: 1, y: 2 })
    cs[1] = Pt { x: 5, y: 6 }
    let held = cs[0]
    let f = () => {
        eprint "go"
        cs[0] = cs[1]
    }
    f()
    if held.x + cs[0].x != 6 { let none: Int? = None; none.unwrap() }
    print "x=${cs[0].x}"
}
`,
}

// A scalar-only Copy record copied into a boxed slot is written in place: neither backend allocates for it.
func TestCopyStoreTickParityAgrees(t *testing.T) {
	t.Parallel()
	for name, src := range copyStoreSources {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := loadSweepModule(t, copyStoreHead+src, "")
			a, err := driver.Analyze(m)
			if err != nil || a.Res.HasErrors() {
				t.Fatalf("analyze: %v", err)
			}
			stores := 0
			for _, f := range a.Prog.Funcs {
				stores += len(a.Prog.CopyStores(f))
			}
			if stores == 0 {
				t.Fatalf("no copy store in the MIR:\n%s", a.Prog.Dump())
			}
			checkArmedTickParity(t, m)
		})
	}
}
