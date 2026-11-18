package instance_test

import (
	"math/big"
	"testing"

	"kigumi/internal/compiler/mir/instance"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

const src = `interface Show { fn show(self) -> String }
type Buf[T: Copy, const N: u128] = { pub v T }
type IB[T: Copy, const N: i128] = { pub v T }
type W[T: Copy] = { pub value T }
fn W[T].show(self) -> String { "w" }
fn W[T].same(self, s: &Show) -> Bool { s is W(_) }
type Ref['a] = { pub r &'a Int }
fn two['a, 'b](x: &'a Int, y: &'b Int) -> Int { x + y }
fn g[T: Copy](v: T) -> Int {
    let c = () => v
    1
}
fn opened(s: &Show) -> Bool { if s is W(x) { x.same(s) } else { false } }
fn plain() -> Int { 1 }
fn pair[A, B](a: A, b: B) -> Int { 1 }
fn wrapped(s: &Show) -> Int {
    if s is W(x) {
        let v = x.value
        let f = () => v
        return 1
    }
    0
}
`

func entity(t *testing.T, r *sem.Result, name string, kind sem.EntityKind) sem.EntityID {
	t.Helper()
	main := r.PackageByPath("main")
	for id := range r.Entities {
		e := r.Entity(sem.EntityID(id))
		if e.Name == name && e.Kind == kind && e.Pkg == main {
			return sem.EntityID(id)
		}
	}
	t.Fatalf("no %s", name)
	return 0
}

func wide(s string) *big.Int {
	v, _ := new(big.Int).SetString(s, 10)
	return v
}

func TestConstWordsAreTwosComplement(t *testing.T) {
	for _, c := range []struct {
		v      string
		lo, hi uint64
	}{
		{"0", 0, 0},
		{"18446744073709551616", 0, 1},
		{"-1", ^uint64(0), ^uint64(0)},
		{"-170141183460469231731687303715884105728", 0, 1 << 63},
		{"340282366920938463463374607431768211455", ^uint64(0), ^uint64(0)},
	} {
		if lo, hi := instance.ConstWords(wide(c.v)); lo != c.lo || hi != c.hi {
			t.Errorf("%s: words %x %x, want %x %x", c.v, lo, hi, c.lo, c.hi)
		}
	}
}

func TestCanonKeepsConstsAtFullWidth(t *testing.T) {
	r := mirfixture.CheckSourceMir(t, src)
	var c instance.Canon
	buf, ib := entity(t, r, "Buf", sem.EntType), entity(t, r, "IB", sem.EntType)
	at := func(ent sem.EntityID, ty sem.TypeID, v string) sem.TypeID {
		return r.Types.Named(ent, []sem.TypeID{sem.TyI64, r.Types.ConstValBig(wide(v), ty)})
	}
	seen := map[sem.TypeID]string{}
	for _, k := range []struct {
		name string
		t    sem.TypeID
	}{
		{"Buf[0]", at(buf, sem.TyU128, "0")},
		{"Buf[2^64]", at(buf, sem.TyU128, "18446744073709551616")},
		{"Buf[2^128-1]", at(buf, sem.TyU128, "340282366920938463463374607431768211455")},
		{"IB[0]", at(ib, sem.TyI128, "0")},
		{"IB[-1]", at(ib, sem.TyI128, "-1")},
		{"IB[i128::MIN]", at(ib, sem.TyI128, "-170141183460469231731687303715884105728")},
	} {
		id := c.Of(r, k.t)
		if prev, dup := seen[id]; dup {
			t.Errorf("%s and %s are one runtime type", prev, k.name)
		}
		seen[id] = k.name
	}
	if c.Of(r, at(buf, sem.TyU128, "18446744073709551616")) != c.Of(r, at(buf, sem.TyU128, "18446744073709551616")) {
		t.Errorf("one instantiation is two runtime types")
	}
}

func TestCanonInternsEqualIdentitiesOnce(t *testing.T) {
	r := mirfixture.CheckSourceMir(t, src)
	var c instance.Canon
	ref, two := entity(t, r, "Ref", sem.EntType), entity(t, r, "two", sem.EntFn)
	tps := r.Fn(two).TypeParams
	a := r.Types.Named(ref, []sem.TypeID{r.Types.Param(tps[0])})
	b := r.Types.Named(ref, []sem.TypeID{r.Types.Param(tps[1])})
	if a == b || c.Of(r, a) != c.Of(r, b) {
		t.Errorf("Ref['a] and Ref['b] are two runtime types")
	}
	w := entity(t, r, "W", sem.EntType)
	if c.Of(r, r.Types.Named(w, []sem.TypeID{sem.TyI64})) == c.Of(r, r.Types.Named(w, []sem.TypeID{sem.TyString})) {
		t.Errorf("W[Int] and W[String] are one runtime type")
	}
}

func TestCanonSpellsAClosureByItsInstantiation(t *testing.T) {
	r := mirfixture.CheckSourceMir(t, src)
	var c instance.Canon
	g := entity(t, r, "g", sem.EntFn)
	var cl sem.EntityID
	for id := range r.Entities {
		if e := r.Entity(sem.EntityID(id)); e.Kind == sem.EntClosure && instance.Enclosing(r, sem.EntityID(id)) == g {
			cl = sem.EntityID(id)
		}
	}
	if cl == 0 {
		t.Fatal("no closure in g")
	}
	tp := r.Fn(g).TypeParams[0]
	source := c.Of(r, r.Types.Closure(cl))
	if instance.Concrete(r, source) {
		t.Errorf("the closure g builds names no parameter")
	}
	ofInt := r.InstantiateClosure(cl, map[sem.EntityID]sem.TypeID{tp: sem.TyI64})
	ofStr := r.InstantiateClosure(cl, map[sem.EntityID]sem.TypeID{tp: sem.TyString})
	again := r.InstantiateClosure(cl, map[sem.EntityID]sem.TypeID{tp: sem.TyI64})
	asInt := r.Types.Subst(source, map[sem.EntityID]sem.TypeID{tp: sem.TyI64})
	switch {
	case c.Of(r, r.Types.Closure(ofInt)) != asInt:
		t.Errorf("g[Int]'s closure differs from g's closure under T = Int")
	case c.Of(r, r.Types.Closure(ofInt)) != c.Of(r, r.Types.Closure(again)):
		t.Errorf("two copies for g[Int] are two closure types")
	case c.Of(r, r.Types.Closure(ofInt)) == c.Of(r, r.Types.Closure(ofStr)):
		t.Errorf("g[Int]'s and g[String]'s closures are one type")
	}
}

func TestCanonSpellsAClosureByTheArgumentsATestOpened(t *testing.T) {
	r := mirfixture.CheckSourceMir(t, src)
	var c instance.Canon
	wrapped := entity(t, r, "wrapped", sem.EntFn)
	var cl sem.EntityID
	for id := range r.Entities {
		if e := r.Entity(sem.EntityID(id)); e.Kind == sem.EntClosure && e.Parent == wrapped {
			cl = sem.EntityID(id)
		}
	}
	if cl == 0 {
		t.Fatal("no closure in wrapped")
	}
	params := instance.Params(r, c.Of(r, r.Types.Closure(cl)))
	if len(params) != 1 {
		t.Fatalf("the closure wrapped builds depends on %d parameters, want the one its test opened", len(params))
	}
	if _, opened := r.OpenedLocal(params[0]); !opened {
		t.Errorf("the closure depends on %s, which no test opened", r.Entity(params[0]).Name)
	}
}

func TestCanonKeepsALambdaCopyAtItsSourcesIdentity(t *testing.T) {
	r := mirfixture.CheckSourceMir(t, src)
	var c instance.Canon
	g := entity(t, r, "g", sem.EntFn)
	var cl sem.EntityID
	for id := range r.Entities {
		if e := r.Entity(sem.EntityID(id)); e.Kind == sem.EntClosure && instance.Enclosing(r, sem.EntityID(id)) == g {
			cl = sem.EntityID(id)
		}
	}
	tp := r.Fn(g).TypeParams[0]
	copied, _ := r.InstantiateClosureCapturing(cl, []sem.TypeID{r.Types.RtType()}, "$rt")
	source := c.Of(r, r.Types.Closure(cl))
	if c.Of(r, r.Types.Closure(copied)) != source {
		t.Errorf("the copy capturing g's environment is another runtime type than its source")
	}
	retyped := r.Types.Subst(source, map[sem.EntityID]sem.TypeID{cl: r.Types.Closure(copied)})
	if retyped != source {
		t.Errorf("retyping the lambda turned its identity %s into %s", r.TypeString(source), r.TypeString(retyped))
	}
	ofInt := r.Types.Subst(source, map[sem.EntityID]sem.TypeID{tp: sem.TyI64, cl: r.Types.Closure(copied)})
	if ofInt == source || len(r.Types.Node(ofInt).Args) != 1 || r.Types.Node(ofInt).Args[0] != sem.TyI64 {
		t.Errorf("the identity under T = Int is %s", r.TypeString(ofInt))
	}
}

func TestErrorAsIsTheStdFunction(t *testing.T) {
	r := mirfixture.CheckSourceMir(t, src)
	pkg := r.PackageByPath("std/error")
	as := r.PackageMember(pkg, "as")
	if as == 0 || !r.IsErrorAs(as) {
		t.Fatalf("std/error.as (%d) is not error.as", as)
	}
	if r.IsErrorAs(entity(t, r, "plain", sem.EntFn)) {
		t.Errorf("a user function is error.as")
	}
}

func TestEnvTypeIsTheReceiverOrTheFunction(t *testing.T) {
	r := mirfixture.CheckSourceMir(t, src)
	w, same := entity(t, r, "W", sem.EntType), entity(t, r, "same", sem.EntFn)
	env, ok := instance.EnvType(r, same)
	if !ok || r.Types.Node(env).Ent != w {
		t.Errorf("W[T].same's environment is %s, want its receiver", r.TypeString(env))
	}
	g := entity(t, r, "g", sem.EntFn)
	env, ok = instance.EnvType(r, g)
	if tp := r.Fn(g).TypeParams[0]; !ok || env != r.Types.Param(tp) || instance.EnvIndex(r, env)[tp] != instance.WholeEnv {
		t.Errorf("g's environment is %s, want its one parameter itself", r.TypeString(env))
	}
	pair := entity(t, r, "pair", sem.EntFn)
	env, ok = instance.EnvType(r, pair)
	if !ok || r.Types.Node(env).Ent != pair || len(r.Types.Node(env).Args) != 2 {
		t.Errorf("pair's environment is %s, want pair applied to A and B", r.TypeString(env))
	}
	if idx := instance.EnvIndex(r, env); idx[r.Fn(pair).TypeParams[1]] != 1 {
		t.Errorf("B sits at %v in pair's environment", idx)
	}
	if _, ok := instance.EnvType(r, entity(t, r, "plain", sem.EntFn)); ok {
		t.Errorf("a non-generic function has an environment")
	}
	if _, ok := instance.EnvType(r, entity(t, r, "two", sem.EntFn)); ok {
		t.Errorf("a function generic only in lifetimes has an environment")
	}
}

func TestTypeTestOpensOnlyParametersOutOfScope(t *testing.T) {
	r := mirfixture.CheckSourceMir(t, src)
	main := r.PackageByPath("main")
	tests := map[instance.Test]int{}
	for f := range r.Files {
		if r.Files[f].Pkg != main {
			continue
		}
		for _, info := range r.Files[f].Pats {
			if info.Kind == sem.PatTypeTest {
				tests[instance.TypeTest(r, info.Type)]++
			}
		}
	}
	if tests[instance.TestExact] != 1 || tests[instance.TestAny] != 2 {
		t.Errorf("type tests by kind %v, want one exact (in W's method) and two open (in plain code)", tests)
	}
}
