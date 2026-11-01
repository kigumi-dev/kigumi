package borrow

import (
	"slices"
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

const sharedStoreSrc = `
import alloc from std/alloc
import mem from std/mem
fn prims(s: alloc.Shared[Future[Int]?], b: alloc.Box[Future[Int]?], c: &mem.UnsafeCell[Future[Int]?], f: Future[Int], g: Future[Int], h: Future[Int]) {
    let mut bb = b
    let mut a: Array[Future[Int]] = Array.empty()
    s.set(Some(f))
    bb.set(Some(g))
    a.push(h)
    let n = s.with((o: &Future[Int]?) => 1)
    unsafe { c.set(None) }
}
fn ints(s: alloc.Shared[Int], n: Int) { s.set(n) }
fn rec(s: alloc.Shared[Future[Int]?], n: Int, f: Future[Int]) {
    if n > 0 { rec(s, n - 1, f) } else { s.set(Some(f)) }
}
fn ra(s: alloc.Shared[Future[Int]?], n: Int, f: Future[Int], k: Future[Int]) { if n > 0 { rb(s, n - 1, k, f) } }
fn rb(s: alloc.Shared[Future[Int]?], n: Int, f: Future[Int], k: Future[Int]) {
    if n > 0 { ra(s, n - 1, f, k) } else { s.set(Some(f)) }
}
fn spin(n: Int, f: Future[Int]) {
    if n > 0 { spin(n - 1, f) }
}
fn viaF[F: Fn[fn(Future[Int]) -> Void]](g: F, f: Future[Int]) { g(f) }
fn sink(f: Future[Int]) {}
fn bound(f: Future[Int]) { viaF(sink, f) }
interface Taker { fn take(mut self, f: Future[Int]) }
type ShT = { cell alloc.Shared[Future[Int]?] }
fn ShT.take(mut self, f: Future[Int]) { self.cell.set(Some(f)) }
type ArrT = { pub mut fs Array[Future[Int]] }
fn ArrT.take(mut self, f: Future[Int]) { self.fs.push(f) }
fn viaIface(o: &mut Taker, f: Future[Int]) { o.take(f) }
fn viaBound[K: Taker](mut k: K, f: Future[Int]) { k.take(f) }
fn applyT[T](f: fn(T, Future[Int]) -> Void, t: T, x: Future[Int]) { f(t, x) }
fn useTakers(mut a: ArrT, s: ShT, f: Future[Int], g: Future[Int]) { viaIface(&mut a, f); viaBound(s, g) }
interface Lone { fn put(self, f: Future[Int]) }
fn viaLone(o: &Lone, f: Future[Int]) { o.put(f) }
`

func sharedStoreProgram(t *testing.T) (*fixture, func(string) *midir.Func) {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(sharedStoreSrc)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatal(res.Render())
	}
	p := buildTypedFixture(res)
	return p, func(name string) *midir.Func {
		g := p.ByEnt[res.PackageMember(res.PackageByPath("main"), name)]
		if g == nil {
			t.Fatalf("no body %s", name)
		}
		return g
	}
}

// storedArgs names the callee parameters each call of g to name may store into shared storage, `?` for an unknown one.
func storedArgs(p *fixture, g *midir.Func, name string) [][]string {
	c := &borrowCheck{p: p.input, state: New(p.input, nil), f: g}
	x := &sinkCtx{g: g, fs: c.bodyFacts(g)}
	var out [][]string
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op != midir.OpCall && in.Op != midir.OpCallValue || in.Ent == 0 || p.R.Entity(in.Ent).Name != name {
				continue
			}
			names := []string{}
			for _, st := range c.sharedStoreArgs(x, in) {
				names = append(names, "?")
				if st.param != 0 {
					names[len(names)-1] = p.R.Entity(st.param).Name
				}
			}
			out = append(out, names)
		}
	}
	return out
}

func TestSharedStoresFollowReceiverMode(t *testing.T) {
	p, body := sharedStoreProgram(t)
	g := body("prims")
	sets := storedArgs(p, g, "set")
	if len(sets) != 3 || !slices.Equal(sets[0], []string{"value"}) || len(sets[1]) != 0 || !slices.Equal(sets[2], []string{"value"}) {
		t.Errorf("set calls store %v, want Shared.set and UnsafeCell.set to store their value and Box.set (mut self) nothing", sets)
	}
	if got := storedArgs(p, g, "push"); len(got) != 1 || len(got[0]) != 0 {
		t.Errorf("Array.push stores %v, want nothing: it takes `mut self`", got)
	}
	if got := storedArgs(p, g, "with"); len(got) != 1 || len(got[0]) != 0 {
		t.Errorf("Shared.with stores %v, want nothing: its callback is non-escaping", got)
	}
	if got := storedArgs(p, body("ints"), "set"); len(got) != 1 || len(got[0]) != 0 {
		t.Errorf("Shared[Int].set stores %v, want nothing: an Int is no shared storage's content", got)
	}
}

func sinkNames(p *fixture, g *midir.Func, bind map[int][]sem.EntityID) []string {
	c := &borrowCheck{p: p.input, state: New(p.input, nil), f: g}
	names := []string{}
	for _, q := range c.sharedSinks(g.Ent, bind) {
		names = append(names, p.R.Entity(q).Name)
	}
	slices.Sort(names)
	return names
}

func TestSharedSinksSettleRecursion(t *testing.T) {
	p, body := sharedStoreProgram(t)
	for range 2 {
		if got := sinkNames(p, body("rec"), nil); !slices.Equal(got, []string{"f"}) {
			t.Errorf("rec sinks %v, want [f] when asked and when remembered", got)
		}
	}
	if got := sinkNames(p, body("ra"), nil); !slices.Equal(got, []string{"f", "k"}) {
		t.Errorf("ra sinks %v, want [f k]: rb stores its f, which ra passes k to", got)
	}
	if got := sinkNames(p, body("rb"), nil); !slices.Equal(got, []string{"f", "k"}) {
		t.Errorf("rb sinks %v, want [f k]", got)
	}
	if got := sinkNames(p, body("spin"), nil); len(got) != 0 {
		t.Errorf("spin sinks %v, want none: it only recurses", got)
	}
}

func TestSharedSinksFollowBinding(t *testing.T) {
	p, body := sharedStoreProgram(t)
	viaF := body("viaF")
	for range 2 {
		if got := sinkNames(p, viaF, nil); !slices.Contains(got, "f") {
			t.Errorf("viaF with g unknown sinks %v, want f in it when asked and when remembered", got)
		}
	}
	sink := p.R.PackageMember(p.R.PackageByPath("main"), "sink")
	if got := sinkNames(p, viaF, map[int][]sem.EntityID{0: {sink}}); len(got) != 0 {
		t.Errorf("viaF with g bound to sink sinks %v, want none", got)
	}
	if got := storedArgs(p, body("bound"), "viaF"); len(got) != 1 || len(got[0]) != 0 {
		t.Errorf("viaF(sink, f) stores %v, want nothing", got)
	}
}

func TestSharedStoresThroughImplementers(t *testing.T) {
	p, body := sharedStoreProgram(t)
	g := body("viaIface")
	c := &borrowCheck{p: p.input, state: New(p.input, nil), f: g}
	req := p.R.Requirement(p.R.PackageMember(p.R.PackageByPath("main"), "Taker"), "take")
	var owners []string
	for _, e := range c.implementers(req) {
		owners = append(owners, p.R.Entity(p.R.Fn(e).Owner).Name)
	}
	slices.Sort(owners)
	if !slices.Equal(owners, []string{"ArrT", "ShT"}) {
		t.Errorf("implementers of Taker.take = %v, want [ArrT ShT], the conformances useTakers settles", owners)
	}
	if got := storedArgs(p, g, "take"); len(got) != 1 || !slices.Equal(got[0], []string{"f"}) {
		t.Errorf("o.take(f) stores %v, want f: ShT keeps it in a Shared", got)
	}
	if got := storedArgs(p, body("viaBound"), "take"); len(got) != 1 || !slices.Equal(got[0], []string{"f"}) {
		t.Errorf("k.take(f) through the bound stores %v, want f", got)
	}
	if got := storedArgs(p, body("viaLone"), "put"); len(got) != 1 || !slices.Equal(got[0], []string{"?", "?"}) {
		t.Errorf("o.put(f) on an interface no type was seen to implement stores %v, want every operand", got)
	}
	if n := p.R.ConformMissesAfterSeal(); n != 0 {
		t.Errorf("%d conformance queries asked after the witness table was sealed", n)
	}
}

func TestUnknownCalleeStoresBesideItsStorage(t *testing.T) {
	p, body := sharedStoreProgram(t)
	if got := sinkNames(p, body("applyT"), nil); !slices.Equal(got, []string{"x"}) {
		t.Errorf("applyT sinks %v, want [x]: f may store x into t, an instance of T", got)
	}
}
