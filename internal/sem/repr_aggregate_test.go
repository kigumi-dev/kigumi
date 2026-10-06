package sem_test

import (
	"testing"

	"kigumi/internal/sem"
)

const aggSrc = `
type Point = { x: Int, y: Int }
type Box = { lo: Point, hi: Point, tight: Bool }
type Named = { at: Point, label: String }
type Thin = { a: f32 }
type Empty = {}
type CHeader layout(C) = { flag: u8, count: u32 }
type Shape = Circle(r: Int) | Dot
type Res = { handle: Int }
let p = Point { x: 1, y: 2 }
let b = Box { lo: p, hi: p, tight: true }
let n = Named { at: p, label: "x" }
let s: Shape = Dot
let t = (1, 2)
`

func TestDirectAggAcceptsScalarRecordsAndTuples(t *testing.T) {
	res := checkRepr(t, aggSrc)
	for _, name := range []string{"Point", "Box"} {
		if !res.DirectAgg(mainType(t, res, name)) {
			t.Errorf("%s is not a direct aggregate", name)
		}
	}
	tuple, ok := res.Types.TupleEntity(2)
	if !ok {
		t.Fatal("no 2-tuple entity")
	}
	if !res.DirectAgg(res.Types.Named(tuple, []sem.TypeID{sem.TyI64, sem.TyBool})) {
		t.Error("a tuple of scalars is not a direct aggregate")
	}
	if res.DirectAgg(res.Types.Named(tuple, []sem.TypeID{sem.TyI64, sem.TyString})) {
		t.Error("a tuple with a String element must stay boxed")
	}
}

func TestDirectAggRejectsCarriersAndLayoutContracts(t *testing.T) {
	res := checkRepr(t, aggSrc)
	for _, name := range []string{"Named", "Thin", "Empty", "CHeader", "Shape"} {
		if res.DirectAgg(mainType(t, res, name)) {
			t.Errorf("%s must not be a direct aggregate", name)
		}
	}
	for _, ty := range []sem.TypeID{sem.TyString, sem.TyBytes, sem.TyI64, sem.TyUnit} {
		if res.DirectAgg(ty) {
			t.Errorf("%s is not a record", res.TypeString(ty))
		}
	}
}

func TestRegScalarMatchesTheRegisterSet(t *testing.T) {
	res := checkRepr(t, aggSrc)
	for _, ty := range []sem.TypeID{sem.TyBool, sem.TyChar, sem.TyF64, sem.TyI64, sem.TyU8, sem.TyIsize} {
		if !res.RegScalar(ty) {
			t.Errorf("%s should be a register scalar", res.TypeString(ty))
		}
	}
	for _, ty := range []sem.TypeID{sem.TyF32, sem.TyString, sem.TyUnit} {
		if res.RegScalar(ty) {
			t.Errorf("%s should not be a register scalar", res.TypeString(ty))
		}
	}
}
