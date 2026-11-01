package driver_test

import "testing"

const staticCopyHead = `import alloc from std/alloc

interface HasX {
    fn setX(mut self, v: Int)
    fn getX(self) -> Int
}

@impl(HasX)
type Pt = { mut x Int, y Int }

fn Pt.setX(mut self, v: Int) { self.x = v }
fn Pt.getX(self) -> Int { self.x }
fn mkPt() -> Pt { Pt { x: 1, y: 2 } }

static origin: Pt = comptime mkPt()

`

var staticCopySources = map[string]string{
	"generic_copy": `fn bump[T: Copy + HasX](v: T) -> Int {
    let mut c = v
    c.setX(7)
    c.getX()
}

fn main() {
    eprint "go"
    if bump(origin) != 7 || origin.x != 1 { let none: Int? = None; none.unwrap() }
    if bump(origin) != 7 || origin.x != 1 { let none: Int? = None; none.unwrap() }
    print "ok"
}
`,
	"shared_try_get": `fn viaShared[T: Copy + HasX](v: T) -> Int {
    let sh = alloc.Shared.new(v)
    let mut c = sh.tryGet().unwrap()
    c.setX(7)
    c.getX()
}

fn main() {
    eprint "go"
    if viaShared(origin) != 7 || origin.x != 1 { let none: Int? = None; none.unwrap() }
    print "ok"
}
`,
	"array_get": `fn viaGet[T: Copy + HasX](v: T) -> Int {
    let xs = Array.of(v)
    let mut c = xs.get(0).unwrap()
    c.setX(8)
    c.getX()
}

fn main() {
    eprint "go"
    if viaGet(origin) != 8 || origin.x != 1 { let none: Int? = None; none.unwrap() }
    print "ok"
}
`,
	"array_first": `fn viaFirst[T: Copy + HasX](v: T) -> Int {
    let xs = Array.of(v)
    let mut c = xs.first().unwrap()
    c.setX(9)
    c.getX()
}

fn main() {
    eprint "go"
    if viaFirst(origin) != 9 || origin.x != 1 { let none: Int? = None; none.unwrap() }
    print "ok"
}
`,
	"concrete": `fn main() {
    eprint "go"
    let xs = Array.of(origin)
    let mut c = xs.get(0).unwrap()
    c.x = 8
    let mut d = xs.first().unwrap()
    d.setX(9)
    let sh = alloc.Shared.new(origin)
    let mut e = sh.tryGet().unwrap()
    e.x = 7
    if c.x + d.x + e.x != 24 || origin.x != 1 || xs[0].x != 1 { let none: Int? = None; none.unwrap() }
    print "ok"
}
`,
}

// A copy of a static with a mut field allocates like native's copy of its ordinary object, one of a read-only
// record of literals is native's shared immortal; armed, since native builds statics before main.
func TestStaticCopyTickParityAgrees(t *testing.T) {
	t.Parallel()
	groups := []struct {
		head    string
		sources map[string]string
	}{{staticCopyHead, staticCopySources}, {staticImmortalHead, staticImmortalSources}}
	for _, g := range groups {
		for name, src := range g.sources {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				checkArmedTickParity(t, loadSweepModule(t, g.head+src, ""))
			})
		}
	}
}
