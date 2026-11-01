package driver_test

const staticImmortalHead = `import alloc from std/alloc

interface HasX {
    fn setX(mut self, v: Int)
    fn getX(self) -> Int
}

@impl(HasX)
type Q = { x Int, y Int }
type In = { a Int, b Int }
@impl(HasX)
type Out = { p In, q Int }
@impl(HasX)
type Pt = { mut x Int, y Int }

fn Q.setX(mut self, v: Int) { self = Q { x: v, y: self.y } }
fn Q.getX(self) -> Int { self.x }
fn Out.setX(mut self, v: Int) { self = Out { p: self.p, q: v } }
fn Out.getX(self) -> Int { self.q }
fn Pt.setX(mut self, v: Int) { self.x = v }
fn Pt.getX(self) -> Int { self.x }

fn mkQ() -> Q { Q { x: 1, y: 2 } }
fn mkOut() -> Out { Out { p: In { a: 1, b: 2 }, q: 3 } }
fn mkPt() -> Pt { Pt { x: 1, y: 2 } }

fn bump[T: Copy + HasX](v: T) -> Int {
    let mut c = v
    c.setX(7)
    c.getX()
}

fn viaGet[T: Copy](v: T) -> T {
    let xs = Array.of(v)
    xs.get(0).unwrap()
}

fn viaShared[T: Copy](v: T) -> T {
    let sh = alloc.Shared.new(v)
    sh.tryGet().unwrap()
}

`

var staticImmortalSources = map[string]string{
	"immortal_flat": `static q: Q = comptime mkQ()

fn main() {
    eprint "go"
    let xs = Array.of(q)
    let c = xs.get(0).unwrap()
    let d = xs.first().unwrap()
    if c.x + d.x != 2 || q.x != 1 { let none: Int? = None; none.unwrap() }
    print "ok"
}
`,
	"immortal_nested": `static o: Out = comptime mkOut()

fn main() {
    eprint "go"
    let xs = Array.of(o)
    let c = xs.get(0).unwrap()
    let d = xs.first().unwrap()
    let ps = Array.of(o.p)
    let e = ps.get(0).unwrap()
    if c.p.a + d.q + e.b != 6 || o.q != 3 { let none: Int? = None; none.unwrap() }
    print "ok"
}
`,
	"immortal_shared": `static q: Q = comptime mkQ()
static o: Out = comptime mkOut()

fn main() {
    eprint "go"
    let sq = alloc.Shared.new(q)
    let c = sq.tryGet().unwrap()
    let so = alloc.Shared.new(o)
    let d = so.tryGet().unwrap()
    if c.y + d.p.a + d.q != 6 || o.q != 3 { let none: Int? = None; none.unwrap() }
    print "ok"
}
`,
	"immortal_generic": `static q: Q = comptime mkQ()
static o: Out = comptime mkOut()

fn main() {
    eprint "go"
    if bump(q) != 7 || bump(o) != 7 || q.x != 1 || o.q != 3 { let none: Int? = None; none.unwrap() }
    let c = viaGet(o)
    let d = viaShared(q)
    if c.p.b + d.y != 4 { let none: Int? = None; none.unwrap() }
    print "ok"
}
`,
	"immortal_beside_mut": `static q: Q = comptime mkQ()
static o: Out = comptime mkOut()
static origin: Pt = comptime mkPt()

fn main() {
    eprint "go"
    let xs = Array.of(q)
    let c = xs.get(0).unwrap()
    let ys = Array.of(origin)
    let mut d = ys.get(0).unwrap()
    d.x = 8
    let e = viaShared(o)
    if bump(origin) + bump(q) != 14 || c.x + d.x + e.p.a != 10 { let none: Int? = None; none.unwrap() }
    if origin.x != 1 || ys[0].x != 1 || q.x != 1 { let none: Int? = None; none.unwrap() }
    print "ok"
}
`,
}
