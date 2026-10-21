package sem

import "kigumi/internal/syntax"

func (r *Result) errBitPattern(f FileID, at syntax.NodeID, t TypeID) bool {
	pointee, raw := r.bitPatternPointee(t)
	if pointee == 0 {
		return false
	}
	what := "Bool"
	if r.underlyingRepr(pointee) == TyChar {
		what = "Char"
	}
	r.errAt(f, at, cCBitPattern, t, what, raw)
	return true
}

// A pointer to Bool stays accepted: Kigumi has no u8 view of a Bool cell to take instead.
func (r *Result) bitPatternPointee(t TypeID) (TypeID, string) {
	tt := r.Types
	u := r.underlyingRepr(t)
	viaPtr := false
	for tt.Kind(u) == KPtr {
		u, viaPtr = r.underlyingRepr(tt.Node(u).Elem), true
	}
	viaCValue := false
	if n := tt.Node(u); n.Kind == KNamed && r.Entities[n.Ent].Name == "CValue" && r.Packages[r.Entities[n.Ent].Pkg].Path == "std/ffi" && len(n.Args) == 1 {
		u, viaCValue = n.Args[0], true
	} else if !viaPtr {
		return 0, ""
	}
	switch r.underlyingRepr(u) {
	case TyBool:
		if viaCValue {
			return u, "u8"
		}
	case TyChar:
		return u, "i32"
	}
	return 0, ""
}
