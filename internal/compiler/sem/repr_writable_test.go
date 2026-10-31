package sem_test

import (
	"testing"

	"kigumi/internal/compiler/sem"
)

const writableSrc = `
import {Shared} from std/alloc

type Named = {
    pub name String
    pub mut n Int
}

type Frozen = {
    pub q Named
}

type Plain = {
    pub a Int
    pub s String
}

type Shape = Circle(r: Int) | Labelled(n: Named)

type Tree = Leaf(n: Named) | Node(kids: FixedArray[Tree, 2])

type Grid = {
    pub cells FixedArray[Int, 4]
}

type Meters = Plain

type Box[T: Copy] = {
    pub v T
}

type Held = {
    pub s Shared[Named]
}

let b = Box { v: 1 }
`

func TestWritableInsideLooksThroughWrappers(t *testing.T) {
	res := checkRepr(t, writableSrc)
	for name, want := range map[string]bool{
		"Named": true, "Frozen": true, "Shape": true, "Tree": true, "Grid": true,
		"Plain": false, "Meters": false, "Held": false,
	} {
		ty := mainType(t, res, name)
		if got := res.WritableInside(ty); got != want {
			t.Errorf("WritableInside(%s) = %v, want %v", name, got, want)
		}
		if res.WritableByParam(ty) {
			t.Errorf("%s does not rest on a type parameter", name)
		}
	}
	named := mainType(t, res, "Named")
	for _, c := range []struct {
		ty   sem.TypeID
		want bool
	}{
		{res.Types.Option(named), true},
		{res.Types.Option(sem.TyI64), false},
		{res.Types.Result(sem.TyI64, named), true},
		{sem.TyString, false},
		{sem.TyBytes, false},
		{res.Types.Ref(named, false), false},
	} {
		if got := res.WritableInside(c.ty); got != c.want {
			t.Errorf("WritableInside(%s) = %v, want %v", res.TypeString(c.ty), got, c.want)
		}
	}
}

func TestWritableInsideRestsOnATypeParameter(t *testing.T) {
	res := checkRepr(t, writableSrc)
	box := res.PackageMember(res.PackageByPath("main"), "Box")
	param := res.Types.Param(res.TypeDecl(box).Params[0])
	for _, ty := range []sem.TypeID{param, res.Types.Option(param), res.Types.Named(box, []sem.TypeID{param})} {
		if !res.WritableInside(ty) || !res.WritableByParam(ty) {
			t.Errorf("%s should be writable only by its type parameter", res.TypeString(ty))
		}
	}
	grid := mainType(t, res, "Grid")
	withGrid := res.Types.Named(box, []sem.TypeID{grid})
	if !res.WritableInside(withGrid) || res.WritableByParam(withGrid) {
		t.Errorf("Box[Grid] is writable whatever the parameter")
	}
}
