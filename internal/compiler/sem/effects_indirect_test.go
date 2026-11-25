package sem_test

import (
	"strings"
	"testing"
)

// indirectRejects are the AUD-10 shapes: a `pure`/`noalloc` body must not run
// a fn value whose declared type fixes no effect, and a call that supplies
// such a value to a deferred `pure?` slot must be rejected.
var indirectRejects = map[string]struct{ src, code string }{
	"param": {`pure fn apply(f: fn() -> Int) -> Int { f() }
fn main() -> Void {
    let g = () => { print "side effect"; 1 }
    print "result=${apply(g)}"
}`, "E901"},
	"paramAlias": {`pure fn apply(f: fn() -> Int) -> Int {
    let g = f
    g()
}
fn main() -> Void {
    print "result=${apply(() => 1)}"
}`, "E901"},
	"noallocParam": {`noalloc fn run(f: fn(u8) -> u8) -> u8 { f(1) }
fn main() -> Void {
    let g = (x: u8) => { let mut a = Array.empty[Int](); a.push(1); x }
    print "result=${run(g)}"
}`, "E908"},
	"noallocFnBound": {`noalloc fn twice[A, F: Fn[fn(A) -> A]](init: A, mut step: F) -> A { step(step(init)) }
fn main() -> Void {
    print "${twice(1, (x) => { let mut a = Array.empty[Int](); a.push(1); x })}"
}`, "E908"},
	"field": {`type R = { f pure? fn() -> Int }
pure fn call(r: &R) -> Int { (r.f)() }
fn main() -> Void {
    let r = R { f: () => { print "SIDE"; 1 } }
    print "result=${call(&r)}"
}`, "E422"},
	"fieldFnItem": {`type R = { f pure? fn() -> Int }
fn plain() -> Int {
    host.files().exists(host.paths().root())
}
pure fn call(r: &R) -> Int { (r.f)() }
fn main() -> Void {
    print "result=${call(&R { f: plain })}"
}`, "E901"},
	"concreteFieldParam": {`type H = { f fn() -> Int }
pure fn call(h: H) -> Int { (h.f)() }
fn main() -> Void {
    print "result=${call(H { f: () => 1 })}"
}`, "E901"},
	"ifaceImpl": {`interface Named { pure fn name(self) -> Int }
@impl(Named)
type Bad = { f fn() -> Int }
pure fn Bad.name(self) -> Int { (self.f)() }
pure fn use[T: Named](t: &T) -> Int { t.name() }
fn main() -> Void {
    let b = Bad { f: () => { print "LAUNDERED IMPURITY"; 1 } }
    print "result=${use(&b)}"
}`, "E901"},
	"ifaceWitness": {`interface Viewer { pure? fn view(self, x: Int) -> Int }
@impl(Viewer)
type Imp = { tag Int }
fn Imp.view(self, x: Int) -> Int {
    print "impure"
    x + self.tag
}
pure fn generic[T: Viewer](t: T, x: Int) -> Int { t.view(x) }
fn main() -> Void {
    print "result=${generic(Imp { tag: 1 }, 1)}"
}`, "E901"},
}

func TestPureRejectsIndirectSlotCalls(t *testing.T) {
	for name, tc := range indirectRejects {
		out := checkBlocking(t, "", tc.src).Render()
		if !strings.Contains(out, tc.code) {
			t.Errorf("%s: expected %s, got:\n%s", name, tc.code, out)
		}
	}
}

// indirectAccepts keep the legitimate shapes working: a declared slot, an
// uninvoked slot, a forwarded slot, std's own trusted accessors, and fresh
// locals that carry their own value.
func TestPureKeepsFixedIndirectCalls(t *testing.T) {
	cases := map[string]string{
		"pureParam": `pure fn apply(f: pure fn() -> Int) -> Int { f() }
fn main() -> Void {
    print "result=${apply(() => 1)}"
}`,
		"keptParam": `pure fn keep(f: fn() -> Int) -> Int {
    _ = f
    2
}
fn main() -> Void {
    print "result=${keep(() => { print "x"; 1 })}"
}`,
		"forwardedParam": `fn apply(f: fn() -> Int) -> Int { f() }
pure fn forward(f: fn() -> Int) -> Int { apply(f) }
fn main() -> Void {
    print "result=${forward(() => 1)}"
}`,
		"pureClosureParam": `pure fn apply(f: pure fn() -> Int) -> Int { f() }
pure fn forward(f: pure fn() -> Int) -> Int { apply(f) }
fn main() -> Void {
    print "result=${forward(() => 1)}"
}`,
		"stdAccessor": `pure fn hof(xs: Array[Int]) -> Array[Int] { xs.map((x) => x + 1) }`,
		"freshLocalClosure": `pure fn go() -> Int {
    let g = () => 1
    g()
}`,
		"freshLocalField": `type R = { f pure? fn() -> Int }
pure fn go() -> Int {
    let r = R { f: () => 1 }
    (r.f)()
}`,
		"pureFieldParam": `type R = { f pure? fn() -> Int }
pure fn call(r: &R) -> Int { (r.f)() }
pure fn go(r: R) -> Int { call(&r) }
fn main() -> Void {
    print "result=${go(R { f: () => 1 })}"
}`,
	}
	for name, src := range cases {
		if out := checkBlocking(t, "", src).Render(); out != "" {
			t.Errorf("%s: expected no diagnostics, got:\n%s", name, out)
		}
	}
}
