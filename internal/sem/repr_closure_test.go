package sem_test

import (
	"testing"

	"kigumi/internal/sem"
)

func mainClosures(res *sem.Result) []sem.TypeID {
	main := res.PackageByPath("main")
	var out []sem.TypeID
	for id := range res.Entities {
		e := res.Entity(sem.EntityID(id))
		if e.Kind == sem.EntClosure && e.Pkg == main {
			out = append(out, res.Types.Closure(sem.EntityID(id)))
		}
	}
	return out
}

func TestDirectClosureNeedsAnInlineEnvironment(t *testing.T) {
	res := checkRepr(t, `
type Point = { x: Int, y: Int }
fn run(p: Point, s: String) -> Int {
    let k = 2
    let o: Int? = Some(1)
    let mut c = 0
    let a = (x: Int) => x + k + p.x + (o || 0)
    let b = () => s.len()
    let d = () => 7
    let e = () => { c = c + 1 }
    let f = (x: Int) => a(x) + 1
    e()
    a(1) + d() + f(1) + c + ((b().toInt()) || 0)
}
`)
	cs := mainClosures(res)
	if len(cs) != 5 {
		t.Fatalf("found %d closures, want 5", len(cs))
	}
	want := []bool{true, false, false, false, true}
	for i, w := range want {
		if got := res.DirectAgg(cs[i]); got != w {
			t.Errorf("closure %d: DirectAgg = %v, want %v (%s)", i+1, got, w, res.TypeString(cs[i]))
		}
	}
	if got := res.AggFields(cs[0]); len(got) != 3 || !res.DirectAgg(got[1]) {
		t.Errorf("the first closure's environment is %v, want Int, Point and Int?", got)
	}
}

func TestDirectFixedArrayFollowsItsElement(t *testing.T) {
	res := checkRepr(t, `
type Point = { x: Int, y: Int }
let a = FixedArray.filled[Int, 4](0)
`)
	fixed := res.LangItem("FixedArray")
	tt := res.Types
	mk := func(elem sem.TypeID, n int64) sem.TypeID {
		return tt.Named(fixed, []sem.TypeID{elem, tt.ConstVal(n, sem.TyUsize)})
	}
	point := mainType(t, res, "Point")
	cases := []struct {
		t    sem.TypeID
		want bool
	}{
		{mk(sem.TyI64, 4), true},
		{mk(point, 3), true},
		{mk(mk(sem.TyBool, 2), 2), true},
		{mk(sem.TyU8, 256), true},
		{mk(sem.TyU8, 4096), true},
		{mk(sem.TyU8, 4097), false},
		{mk(sem.TyU8, 1<<40), false},
		{mk(sem.TyI64, 512), true},
		{mk(sem.TyI64, 513), false},
		{mk(point, 256), true},
		{mk(point, 257), false},
		{mk(mk(sem.TyI64, 16), 32), true},
		{mk(mk(sem.TyI64, 16), 33), false},
		{mk(mk(sem.TyI64, 1024), 64), false},
		{mk(sem.TyString, 2), false},
		{mk(sem.TyF32, 2), true},
	}
	for _, c := range cases {
		if got := res.DirectAgg(c.t); got != c.want {
			t.Errorf("DirectAgg(%s) = %v, want %v", res.TypeString(c.t), got, c.want)
		}
	}
	if res.Repr(mk(sem.TyString, 2)) != sem.ReprDirect {
		t.Error("a FixedArray's elements are its own storage, whatever they hold")
	}
}

func TestRecursionThroughFixedArrayStaysBoxed(t *testing.T) {
	res := checkRepr(t, `
type Tree = Leaf(v: Int) | Node(kids: FixedArray[Tree, 2])
type Rose = { tag Int, kids FixedArray[Rose, 2] }
`)
	for _, name := range []string{"Tree", "Rose"} {
		if res.DirectAgg(mainType(t, res, name)) {
			t.Errorf("%s reaches itself through a FixedArray, so it must stay boxed", name)
		}
	}
}

func TestDirectAggFollowsALateCellCapture(t *testing.T) {
	res := checkRepr(t, `
type Holder[F] = { f: F, tag: Int }
fn id[T](x: T) -> T { x }
fn run() -> Int {
    let mut n = 1
    let f = () => n
    let h = id(Holder { f: f, tag: 2 })
    n = 5
    h.tag
}
`)
	cs := mainClosures(res)
	if len(cs) != 1 {
		t.Fatalf("found %d closures, want 1", len(cs))
	}
	holder := res.Types.Named(res.PackageMember(res.PackageByPath("main"), "Holder"), []sem.TypeID{cs[0]})
	if res.DirectAgg(cs[0]) || res.DirectAgg(holder) {
		t.Errorf("a closure over a cell, and a record holding it, must not be direct (%s)", res.TypeString(holder))
	}
}
