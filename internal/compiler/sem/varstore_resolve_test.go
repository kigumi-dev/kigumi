package sem

import (
	"math/rand/v2"
	"testing"
)

func testVarStore() *varStore {
	return newVarStore(&Result{Types: newTypeTable(64)})
}

func TestResolveBindingsAndRollback(t *testing.T) {
	v := testVarStore()
	tt := v.r.Types
	a, b := v.fresh(0, "a"), v.fresh(0, "b")
	shape := tt.Fn([]TypeID{tt.Ref(a, false)}, tt.Named(99, []TypeID{b}), 0, false)
	if got := v.resolve(shape); got != shape {
		t.Fatal("unbound shape changed")
	}
	mark := v.mark()
	v.bind(0, tt.Named(98, []TypeID{b}))
	want := tt.Fn([]TypeID{tt.Ref(tt.Named(98, []TypeID{b}), false)}, tt.Named(99, []TypeID{b}), 0, false)
	if got := v.resolve(shape); got != want {
		t.Fatalf("partial binding = %d, want %d", got, want)
	}
	v.bind(1, TyI64)
	want = tt.Fn([]TypeID{tt.Ref(tt.Named(98, []TypeID{TyI64}), false)}, tt.Named(99, []TypeID{TyI64}), 0, false)
	if got := v.resolve(shape); got != want {
		t.Fatalf("transitive binding = %d, want %d", got, want)
	}
	v.rollback(mark)
	if got := v.resolve(shape); got != shape {
		t.Fatal("rollback retained a trial binding")
	}
	if !v.unifiable(a, TyBool) || v.resolve(shape) != shape {
		t.Fatal("unifiability trial changed the type")
	}
	left := tt.Fn([]TypeID{a, a}, TyUnit, 0, false)
	right := tt.Fn([]TypeID{TyI64, TyBool}, TyUnit, 0, false)
	if v.unify(left, right) || v.resolve(a) != a || v.resolve(shape) != shape {
		t.Fatal("failed unification retained a partial binding")
	}
	v.poisonFrom(0)
	want = tt.Fn([]TypeID{tt.Ref(TyPoison, false)}, tt.Named(99, []TypeID{TyPoison}), 0, false)
	if got := v.resolve(shape); got != want {
		t.Fatalf("poisoned type = %d, want %d", got, want)
	}
}

func TestResolveReferenceLifetimeAndEscaping(t *testing.T) {
	v := testVarStore()
	tt := v.r.Types
	a, b := v.fresh(0, "type"), v.fresh(0, "lifetime")
	ref := tt.WithRefLifetime(tt.Ref(a, true), b)
	sig := tt.WithEscParams(tt.Fn([]TypeID{a, ref}, TyUnit, 0, false), string([]byte{1}))
	v.resolve(sig)
	mark := v.mark()
	fn := tt.Fn(nil, TyBool, 0, false)
	v.bind(0, fn)
	v.bind(1, tt.Param(99))
	want := tt.Fn([]TypeID{tt.WithEscaping(fn), tt.WithRefLifetime(tt.Ref(fn, true), tt.Param(99))}, TyUnit, 0, false)
	if got := v.resolve(sig); got != want {
		t.Fatalf("qualified signature = %d, want %d", got, want)
	}
	v.rollback(mark)
	if got := v.resolve(sig); got != sig {
		t.Fatal("rollback retained a lifetime or escaping function")
	}
	v.poison(ref)
	if got := v.resolve(ref); got != tt.WithRefLifetime(tt.Ref(TyPoison, true), b) {
		t.Fatal("poison did not invalidate a cached reference")
	}
}

func TestResolveSettledDefaults(t *testing.T) {
	v := testVarStore()
	tt := v.r.Types
	a, b, lifetime := v.fresh(0, "int"), v.fresh(0, "float"), v.fresh(0, "lifetime")
	shape := tt.Fn([]TypeID{a, b}, tt.WithRefLifetime(tt.Ref(TyI64, false), lifetime), 0, false)
	v.resolve(shape)
	v.pending[0], v.pending[1], v.lifetimeDefault[2] = 1, 2, tt.Param(99)
	v.settle(&checker{r: v.r, vars: v})
	want := tt.Fn([]TypeID{TyI64, TyF64}, tt.WithRefLifetime(tt.Ref(TyI64, false), tt.Param(99)), 0, false)
	if got := v.resolve(shape); got != want {
		t.Fatalf("settled type = %d, want %d", got, want)
	}
}

func TestResolveMatchesUncachedAcrossBindingChanges(t *testing.T) {
	v := testVarStore()
	tt := v.r.Types
	var variables []TypeID
	for range 8 {
		variables = append(variables, v.fresh(0, "T"))
	}
	rng := rand.New(rand.NewPCG(17, 41))
	roots := append([]TypeID(nil), variables...)
	for range 40 {
		a, b := roots[rng.IntN(len(roots))], roots[rng.IntN(len(roots))]
		switch rng.IntN(3) {
		case 0:
			roots = append(roots, tt.Named(99, []TypeID{a, b}))
		case 1:
			roots = append(roots, tt.WithRefLifetime(tt.Ref(a, true), b))
		case 2:
			roots = append(roots, tt.WithEscParams(tt.Fn([]TypeID{a}, b, 0, false), string([]byte{1})))
		}
	}
	for step := range 200 {
		i := rng.IntN(len(variables))
		candidates := []TypeID{0, TyI64, TyBool, tt.Fn(nil, TyUnit, 0, false)}
		if i+1 < len(variables) {
			b := variables[i+1+rng.IntN(len(variables)-i-1)]
			candidates = append(candidates, b, tt.Ref(b, false), tt.Named(99, []TypeID{b}))
		}
		v.setBinding(uint32(i), candidates[rng.IntN(len(candidates))])
		for _, root := range roots {
			if got, want := v.resolve(root), resolveUncached(v, root); got != want {
				t.Fatalf("step %d, type %d: cached %d, uncached %d", step, root, got, want)
			}
		}
	}
}

func resolveUncached(v *varStore, t TypeID) TypeID {
	tt := v.r.Types
	if t == 0 || !tt.ContainsVar(t) {
		return t
	}
	n := tt.Node(t)
	if n.Kind == KVar {
		if b := v.binding[n.Var]; b != 0 {
			return resolveUncached(v, b)
		}
		return t
	}
	args := make([]TypeID, len(n.Args))
	for i, a := range n.Args {
		args[i] = resolveUncached(v, a)
	}
	n.Args, n.Elem, n.Lifetime = args, resolveUncached(v, n.Elem), resolveUncached(v, n.Lifetime)
	return tt.Intern(tt.absorbEscMarks(n))
}
