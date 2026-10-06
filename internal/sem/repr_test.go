package sem_test

import (
	"testing"

	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

func checkRepr(t *testing.T, src string) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", res.Render())
	}
	return res
}

func mainType(t *testing.T, res *sem.Result, name string) sem.TypeID {
	t.Helper()
	ent := res.PackageMember(res.PackageByPath("main"), name)
	if ent == 0 {
		t.Fatalf("no such type %q in main", name)
	}
	return res.Types.Named(ent, nil)
}

func fieldType(t *testing.T, res *sem.Result, owner sem.EntityID, name string) sem.TypeID {
	t.Helper()
	fld := res.FindField(owner, name)
	if fld == 0 {
		t.Fatalf("no such field %q", name)
	}
	return res.Field(fld).Type
}

func TestReprScalarsAndRefsAreDirect(t *testing.T) {
	res := checkRepr(t, `
type Pair = { x: Int, y: Int }
let p = Pair { x: 1, y: 2 }
`)
	for _, ty := range []sem.TypeID{sem.TyI64, sem.TyBool, sem.TyChar, sem.TyUnit, sem.TyF64} {
		if res.Repr(ty) != sem.ReprDirect {
			t.Errorf("Repr(%s) = carrier, want direct", res.TypeString(ty))
		}
	}
	if res.Repr(res.Types.Ref(sem.TyString, false)) != sem.ReprDirect {
		t.Error("a borrowed &String view must be direct, even though String itself is a carrier")
	}
	if got := res.Repr(mainType(t, res, "Pair")); got != sem.ReprDirect {
		t.Errorf("Repr(Pair) = %v, want direct", got)
	}
}

func TestReprCarriers(t *testing.T) {
	res := checkRepr(t, `
let s: String = "x"
`)
	for _, ty := range []sem.TypeID{sem.TyString, sem.TyBytes} {
		if res.Repr(ty) != sem.ReprCarrier {
			t.Errorf("Repr(%s) = direct, want carrier", res.TypeString(ty))
		}
	}
	arr := res.Types.Named(res.Types.ArrayEnt(), []sem.TypeID{sem.TyI64})
	if res.Repr(arr) != sem.ReprCarrier {
		t.Error("Array[Int] must be a carrier")
	}
}

func TestReprOrdinaryAdtIsDirect(t *testing.T) {
	res := checkRepr(t, `
type Shape = Circle(r: Int) | Square(side: Int)
let s = Circle(1)
`)
	if got := res.Repr(mainType(t, res, "Shape")); got != sem.ReprDirect {
		t.Errorf("Repr(Shape) = %v, want direct", got)
	}
}

func TestReprRecordWithCarrierFieldStaysDirect(t *testing.T) {
	res := checkRepr(t, `
type Named = { tag: String, n: Int }
let v = Named { tag: "x", n: 1 }
`)
	named := mainType(t, res, "Named")
	if got := res.Repr(named); got != sem.ReprDirect {
		t.Errorf("Repr(Named) = %v, want direct", got)
	}
	ent := res.Types.Node(named)
	if got := res.Repr(fieldType(t, res, ent.Ent, "tag")); got != sem.ReprCarrier {
		t.Errorf("Repr(Named.tag) = %v, want carrier", got)
	}
}

func checkReprRejects(t *testing.T, src string) {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if !res.HasErrors() {
		t.Fatalf("expected an error, got none:\n%s", src)
	}
}

func TestReprByValueRecursionThroughTupleIsRejected(t *testing.T) {
	checkReprRejects(t, `
type NodeTuple = LeafC(v: Int) | BranchC(kid: (Int, NodeTuple))
`)
	checkReprRejects(t, `
type RecTuple = { next (Int, RecTuple) }
`)
}

func TestReprByValueRecursionThroughDefinedIsRejected(t *testing.T) {
	checkReprRejects(t, `
type Newtype = (Int, Newtype)
`)
}
