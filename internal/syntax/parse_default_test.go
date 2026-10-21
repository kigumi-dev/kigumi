package syntax_test

import (
	"testing"

	"kigumi/internal/syntax"
)

func TestDefaultParamThunks(t *testing.T) {
	tree := parseOK(t, "fn f(a: Int, b: Int = a, c: Int = b + a) -> Int { a }\nfn P.m(self, x: Int = self.n) -> Int { x }\nfn P.k(mut self, x: Int = 1) -> Int { x }\nfn g(a: Int) -> Int { a }\n")
	decls := tree.List(tree.Root)
	thunkOf := func(decl, param int) syntax.FnDeclView {
		p := decls.At(decl).AsFnDecl().Params().At(param).AsParam()
		if p.Thunk() == 0 {
			t.Fatalf("decl %d param %d has no thunk", decl, param)
		}
		id := syntax.NodeID(p.Thunk())
		if !tree.IsDefaultThunk(id) {
			t.Fatalf("decl %d param %d: thunk %d is not a default thunk", decl, param, id)
		}
		return tree.View(id).AsFnDecl()
	}
	if got := decls.At(0).AsFnDecl().Params().At(0).AsParam().Thunk(); got != 0 {
		t.Fatalf("a parameter without a default has thunk %d", got)
	}
	if n := thunkOf(0, 1).Params().Len(); n != 1 {
		t.Fatalf("first default sees %d parameters, want a", n)
	}
	c := thunkOf(0, 2)
	if c.Params().Len() != 2 || c.Params().At(1).AsParam().Name().Text() != "b" {
		t.Fatalf("second default sees %d parameters", c.Params().Len())
	}
	if m := thunkOf(1, 1); !m.Recv().Valid() || m.Params().Len() != 1 || m.Params().At(0).AsParam().Flags()&syntax.FlagSelf == 0 {
		t.Fatalf("a method default does not take self")
	}
	if k := thunkOf(2, 1); k.Recv().Valid() || k.Params().Len() != 0 {
		t.Fatalf("a `mut self` method default takes %d parameters with receiver %v", k.Params().Len(), k.Recv().Valid())
	}
	if tree.IsDefaultThunk(decls.At(3).ID()) {
		t.Fatal("a declaration is not a default thunk")
	}
}

func TestDefaultThunkOfGenericAndFallible(t *testing.T) {
	tree := parseOK(t, "fn g[T: Eq](x: T, y: Int = 1) -> T { x }\nfn Box[T].m(self, k: Int = f()?) -> Int { k }\nfn f() -> Int! { Ok(1) }\n")
	decls := tree.List(tree.Root)
	thunk := func(decl int) syntax.FnDeclView {
		return tree.View(syntax.NodeID(decls.At(decl).AsFnDecl().Params().Last().AsParam().Thunk())).AsFnDecl()
	}
	g := decls.At(0).AsFnDecl()
	gt := thunk(0)
	if !gt.Generics().Valid() || gt.Generics().ID() == g.Generics().ID() {
		t.Fatal("the thunk of a generic function needs its own copy of the generics")
	}
	if gt.Params().At(0).AsParam().Type().ID() == g.Params().At(0).AsParam().Type().ID() {
		t.Fatal("the thunk shares a parameter type node with its function")
	}
	if gt.Mods()&syntax.ModDefaultTry != 0 {
		t.Fatal("a default without `?` marks its thunk fallible")
	}
	mt := thunk(1)
	if mt.Mods()&syntax.ModDefaultTry == 0 || mt.Ret().Kind() != syntax.TypeResult {
		t.Fatal("a default with `?` must return `T!`")
	}
	if !mt.Recv().Valid() || mt.Recv().ID() == decls.At(1).AsFnDecl().Recv().ID() {
		t.Fatal("the thunk of a method on a generic type needs a copy of the receiver binder")
	}
}
