package mir

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

func TestBodyRolesCoverEveryOpcode(t *testing.T) {
	for n := range 256 {
		op := midir.Opcode(n)
		s, exists := op.Shape()
		if !exists {
			continue
		}
		role, ok := bodyRoles[op]
		switch {
		case !ok:
			t.Errorf("%s has no bodyFacts role", op)
		case (role == roleCarry || role == roleFresh) && !s.Result:
			t.Errorf("%s is given a result role but defines no result", op)
		}
	}
}

const bodyFlowSrc = `
type Bx = { pub mut f fn(Int) -> Int }
type Wrap = { pub f fn(Int) -> Int }
fn Bx.me(mut self) -> &mut Bx { &mut self }
fn Bx.put(mut self, f: escaping fn(Int) -> Int) { self.f = f }
fn plain(n: Int) -> Int { n }
fn target(n: Int) -> Int { n + 1 }
fn pick() -> fn(Int) -> Int { target }
fn setF(b: &mut Bx, f: escaping fn(Int) -> Int) { b.f = f }
fn carried(f: escaping fn(Int) -> Int) -> Int {
    let a = Array.of(f)
    let copied = a
    let boxed = Wrap { f: f }
    let k = pick()
    (copied.len().toInt() || 0) + (boxed.f)(1) + k(1)
}
fn storedDirect() -> Int {
    let mut b = Bx { f: plain }
    b.f = target
    (b.f)(1)
}
fn storedThroughReturnedRef() -> Int {
    let mut b = Bx { f: plain }
    let m = b.me()
    m.f = target
    (b.f)(1)
}
fn storedThroughBranch(c: Bool) -> Int {
    let mut b1 = Bx { f: plain }
    let mut b2 = Bx { f: plain }
    let m = if c { &mut b1 } else { &mut b2 }
    m.f = target
    (b1.f)(1) + (b2.f)(1)
}
fn storedByCapture() -> Int {
    let mut g: fn(Int) -> Int = plain
    let st = () => { g = target }
    st()
    g(1)
}
fn storedThroughMutArg() -> Int {
    let mut b = Bx { f: plain }
    setF(&mut b, target)
    (b.f)(1)
}
fn storedThroughWriteBack() -> Int {
    let mut b = Bx { f: plain }
    b.put(target)
    (b.f)(1)
}
type Hd = { pub mut b Bx }
fn outStore(b: &mut Bx) { b.f = target }
fn Bx.setTarget(mut self) { self.f = target }
fn pickRef['a](h: &'a mut Hd) -> &'a mut Bx {
    h.b.f = target
    &mut h.b
}
fn calleeStoresParam() -> Int {
    let mut b = Bx { f: plain }
    outStore(&mut b)
    (b.f)(1)
}
fn calleeStoresReceiver() -> Int {
    let mut b = Bx { f: plain }
    b.setTarget()
    (b.f)(1)
}
fn calleeStoresAndReturnsRef() -> Int {
    let mut hd = Hd { b: Bx { f: plain } }
    let m = pickRef(&mut hd)
    (m.f)(1)
}
fn closureStoresParam() -> Int {
    let mut b = Bx { f: plain }
    let st = (x: &mut Bx) => { x.f = target }
    st(&mut b)
    (b.f)(1)
}
fn recursingCapture(n: Int) -> Int {
    let mut g: fn(Int) -> Int = plain
    let st = () => {
        g = target
        let k = recursingCapture(0)
    }
    st()
    g(1)
}
fn unseenStoresParam(g: fn(&mut Bx) -> Void) -> Int {
    let mut b = Bx { f: plain }
    g(&mut b)
    (b.f)(1)
}
`

func bodyFlowProgram(t *testing.T) (*Program, func(string) sem.EntityID) {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(bodyFlowSrc)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatal(res.Render())
	}
	pkg := res.PackageByPath("main")
	return Build(res), func(name string) sem.EntityID { return res.PackageMember(pkg, name) }
}

// TestBodyFactsFollowsEveryRole checks each role on a body: carrying, a callable call result, pinned stores, capture
// stores, `&mut` and write-back calls, and a callee's stores into its `&mut` parameter, receiver and returned reference.
func TestBodyFactsFollowsEveryRole(t *testing.T) {
	p, member := bodyFlowProgram(t)
	holds := func(fn, local string, fact uint32) bool {
		t.Helper()
		for _, f := range p.Funcs {
			if f.Ent == 0 || p.R.Entity(f.Ent).Name != fn {
				continue
			}
			c := &borrowCheck{p: p, f: f}
			fs := c.bodyFacts(f)
			for l, loc := range f.Locals {
				if loc.Name == local {
					return fs[midir.LocalID(l)][fact]
				}
			}
			t.Fatalf("%s has no local %s", fn, local)
		}
		t.Fatalf("no body %s", fn)
		return false
	}
	param := uint32(p.R.Fn(member("carried")).Params[0])
	target := uint32(member("target"))
	for _, tc := range []struct {
		fn, local string
		fact      uint32
	}{
		{"carried", "copied", param},
		{"carried", "boxed", param},
		{"carried", "k", opaqueFact},
		{"storedDirect", "b", target},
		{"storedThroughReturnedRef", "b", target},
		{"storedThroughBranch", "b1", target},
		{"storedThroughBranch", "b2", target},
		{"storedByCapture", "g", target},
		{"storedThroughMutArg", "b", target},
		{"storedThroughWriteBack", "b", target},
		{"calleeStoresParam", "b", target},
		{"calleeStoresReceiver", "b", target},
		{"calleeStoresAndReturnsRef", "hd", target},
		{"calleeStoresAndReturnsRef", "m", target},
		{"closureStoresParam", "b", target},
		{"unseenStoresParam", "b", opaqueFact},
	} {
		if !holds(tc.fn, tc.local, tc.fact) {
			t.Errorf("%s: %s does not hold fact %d", tc.fn, tc.local, tc.fact)
		}
	}
}
