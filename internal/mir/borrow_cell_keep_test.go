package mir

import (
	"slices"
	"testing"

	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

const keepSrc = `
import task from std/task
import alloc from std/alloc
import ffi from std/ffi
fn target(n: Int) -> Int { n + 1 }
fn kinds(a: alloc.Shared[Future[Int]], b: alloc.Shared[Int], c: Future[Int]?, d: Array[Future[Int]], e: ffi.HandleTable[Future[Int]], f: fn(Array[fn(Future[Int]) -> Void]) -> Void, g: alloc.Shared[fn(Int) -> Int], h: &alloc.Shared[Future[Int]]) {}
fn prims(s: alloc.Shared[Future[Int]?], pool: task.BlockingPool) -> Int {
    s.set(None)
    let n = s.with((o: &Future[Int]?) => 1)
    let w = pool.run(() => 1)
    n
}
fn keepFn(s: alloc.Shared[(fn(Int) -> Int)?]) { s.set(Some(target)) }
fn viaHelper() -> Int {
    let s: alloc.Shared[(fn(Int) -> Int)?] = alloc.Shared.new(None)
    keepFn(s.clone())
    0
}
fn viaMethodValue() -> Int {
    let s: alloc.Shared[(fn(Int) -> Int)?] = alloc.Shared.new(None)
    let set = s.set
    set(Some(target))
    0
}
fn classes(p: alloc.Shared[Future[Int]?], f: Future[Int]) {
    let s: alloc.Shared[Future[Int]?] = alloc.Shared.new(None)
    let s2 = s.clone()
    let apart: alloc.Shared[Future[Int]?] = alloc.Shared.new(None)
    s.set(None)
    p.set(Some(f))
    let k = s2
    let a = apart
}
type Store = { s alloc.Shared[Future[Int]?] }
fn same(s: alloc.Shared[Future[Int]?]) -> alloc.Shared[Future[Int]?] { s }
fn byClone(p: alloc.Shared[Future[Int]?], f: Future[Int]) { let t = p.clone(); t.set(Some(f)) }
fn byLet(p: alloc.Shared[Future[Int]?], f: Future[Int]) { let t = p; t.set(Some(f)) }
fn byHelper(p: alloc.Shared[Future[Int]?], f: Future[Int]) { let t = same(p); t.set(Some(f)) }
fn byField(st: Store, f: Future[Int]) { st.s.set(Some(f)) }
fn Store.bySelf(self, f: Future[Int]) { self.s.set(Some(f)) }
`

func keepProgram(t *testing.T) (*Program, func(string) *Func) { return srcProgram(t, keepSrc) }

// srcProgram builds src as package main and finds a body by name, a method's too.
func srcProgram(t *testing.T, src string) (*Program, func(string) *Func) {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatal(res.Render())
	}
	p := Build(res)
	return p, func(name string) *Func {
		if g := p.ByEnt[res.PackageMember(res.PackageByPath("main"), name)]; g != nil {
			return g
		}
		for e, g := range p.ByEnt {
			if g != nil && res.Entity(e).Name == name && res.Entity(e).Kind == sem.EntFn && res.Fn(e).Recv != sem.RecvNone {
				return g
			}
		}
		t.Fatalf("no body %s", name)
		return nil
	}
}

func localNamed(t *testing.T, g *Func, name string) LocalID {
	t.Helper()
	for l, loc := range g.Locals {
		if loc.Name == name {
			return LocalID(l)
		}
	}
	t.Fatalf("no local %s", name)
	return 0
}

func calls(p *Program, g *Func, name string) []Inst {
	var out []Inst
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op == OpCall && in.Ent != 0 && p.R.Entity(in.Ent).Name == name {
				out = append(out, in)
			}
		}
	}
	return out
}

func TestSharedStorageFollowsLayout(t *testing.T) {
	p, body := keepProgram(t)
	g := body("kinds")
	c := &borrowCheck{p: p, f: g}
	want := map[string]bool{"a": true, "b": false, "c": false, "d": true, "e": true, "f": false, "g": true, "h": true}
	for name, shared := range want {
		if got := c.sharedStorage(g.Locals[localNamed(t, g, name)].Type); got != shared {
			t.Errorf("sharedStorage(%s) = %v, want %v", name, got, shared)
		}
	}
}

func TestPrimSummaryKeepsIntoSharedStorage(t *testing.T) {
	p, body := keepProgram(t)
	g := body("prims")
	c := &borrowCheck{p: p, f: g}
	set, with, run := calls(p, g, "set")[0].Ent, calls(p, g, "with")[0].Ent, calls(p, g, "run")[0].Ent
	params := c.declaredParams(set)
	if got := c.primSummary(set).stores[params[0]]; !slices.Equal(got, []uint32{uint32(params[1])}) {
		t.Errorf("Shared.set keeps %v into its receiver, want its value", got)
	}
	if got := c.primSummary(with).stores; len(got) != 0 {
		t.Errorf("Shared.with keeps %v, want nothing: its callback is non-escaping", got)
	}
	if got := c.primSummary(run).stores; len(got) != 0 {
		t.Errorf("BlockingPool.run keeps %v, want nothing: the pool is no storage of the closure's", got)
	}
}

func TestBodyFactsFollowSharedStores(t *testing.T) {
	p, body := keepProgram(t)
	target := uint32(p.R.PackageMember(p.R.PackageByPath("main"), "target"))
	for _, fn := range []string{"keepFn", "viaHelper", "viaMethodValue"} {
		g := body(fn)
		c := &borrowCheck{p: p, f: g}
		if !c.bodyFacts(g)[localNamed(t, g, "s")][target] {
			t.Errorf("%s: s does not hold target", fn)
		}
	}
}

func TestKeeperClassLinksEarlierCopies(t *testing.T) {
	p, body := keepProgram(t)
	g := body("classes")
	c := &borrowCheck{p: p, f: g}
	sets := calls(p, g, "set")
	local, param := c.keeperOf(sets[0].Args[0]), c.keeperOf(sets[1].Args[0])
	if !slices.Contains(local.locals, localNamed(t, g, "s2")) || slices.Contains(local.locals, localNamed(t, g, "apart")) || local.escapes {
		t.Errorf("class of s = %v (escapes %v), want s2 in it, apart out, no escape", local.locals, local.escapes)
	}
	if !param.escapes {
		t.Errorf("a store into parameter p does not escape the body")
	}
}

func TestKeeperClassReachesFirstParam(t *testing.T) {
	p, body := keepProgram(t)
	for _, fn := range []string{"byClone", "byLet", "byHelper", "byField", "bySelf"} {
		g := body(fn)
		c := &borrowCheck{p: p, f: g}
		k := c.keeperOf(calls(p, g, "set")[0].Args[0])
		if !k.escapes || !slices.Contains(k.locals, g.Params[0]) || !slices.Contains(c.sharedPlaces(calls(p, g, "set")[0].Args[0]), c.spawnScope()) {
			t.Errorf("%s: class %v (escapes %v), want the first parameter in it and the task scope kept", fn, k.locals, k.escapes)
		}
	}
}
