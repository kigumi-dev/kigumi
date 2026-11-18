package sem_test

import (
	"testing"

	"kigumi/internal/compiler/sem"
)

const ctorKeySrc = `
interface Show { fn show(self) -> String }
type View[T] = { pub v T }
fn View[T].show(self) -> String { "v" }
fn mk(n: Int) -> fn() -> Int { () => n }
`

func TestCtorKeyIgnoresNumbering(t *testing.T) {
	plain := checkRepr(t, ctorKeySrc)
	padded := checkRepr(t, "type Pad1 = { pub a Int }\ntype Pad2 = { pub b Pad1 }\nfn pad() -> Int { let f = () => 1; f() }\n"+ctorKeySrc)
	type pick func(*testing.T, *sem.Result) sem.TypeID
	picks := map[string]pick{
		"named": func(t *testing.T, r *sem.Result) sem.TypeID { return mainType(t, r, "View") },
		"iface": func(t *testing.T, r *sem.Result) sem.TypeID {
			return r.Types.Iface(r.PackageMember(r.PackageByPath("main"), "Show"), nil)
		},
		"closure": func(t *testing.T, r *sem.Result) sem.TypeID { return r.Types.Closure(closureIn(t, r, "mk")) },
		"prim":    func(t *testing.T, r *sem.Result) sem.TypeID { return sem.TyI64 },
	}
	seen := map[string]string{}
	for name, p := range picks {
		a, b := p(t, plain), p(t, padded)
		if name != "prim" && a == b {
			t.Fatalf("%s: the padding left the type numbering alone (%d)", name, a)
		}
		ka, kb := plain.CtorKey(a), padded.CtorKey(b)
		if ka != kb {
			t.Errorf("%s: constructor key %q under one numbering, %q under another", name, ka, kb)
		}
		if other, dup := seen[ka]; dup {
			t.Errorf("%s and %s share the constructor key %q", name, other, ka)
		}
		seen[ka] = name
	}
}

func closureIn(t *testing.T, r *sem.Result, fn string) sem.EntityID {
	t.Helper()
	parent := r.PackageMember(r.PackageByPath("main"), fn)
	for id := range r.Entities {
		if e := r.Entity(sem.EntityID(id)); e.Kind == sem.EntClosure && e.Parent == parent {
			return sem.EntityID(id)
		}
	}
	t.Fatalf("no closure in %s", fn)
	return 0
}
