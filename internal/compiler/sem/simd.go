package sem

import (
	"fmt"

	"kigumi/internal/compiler/syntax"
)

// SimdVecBits is the width every Vec[T, N] instantiates to: one vector register.
const SimdVecBits = 128

// SimdElem reports whether t is a lane type a 128-bit vector holds on every
// supported target, or 0 when it is not.
func (r *Result) SimdElem(t TypeID) bool {
	switch t {
	case TyI8, TyU8, TyI16, TyU16, TyI32, TyU32, TyI64, TyU64, TyF32, TyF64:
		return true
	}
	return false
}

// SimdVecShape recognizes std/simd's Vec[elem, N] with N a literal length:
// the only const-generic instance a 128-bit vector can name.
func (r *Result) SimdVecShape(t TypeID) (elem TypeID, n int64, ok bool) {
	tt := r.Types
	if t == 0 || tt.Kind(t) != KNamed {
		return 0, 0, false
	}
	node := tt.Node(t)
	ent := r.langItem(0, 0, "Vec")
	if ent == 0 || node.Ent != ent || len(node.Args) != 2 || !tt.IsConst(node.Args[1]) {
		return 0, 0, false
	}
	v, _ := tt.ConstValue(node.Args[1])
	return node.Args[0], v, true
}

// SimdVecProblem says why t cannot be a 128-bit vector, or "" when it can or
// is not a Vec at all. An open type parameter or const is left to the call
// site that instantiates it.
func (r *Result) SimdVecProblem(t TypeID) string {
	elem, n, ok := r.SimdVecShape(t)
	if !ok || elem == 0 {
		return ""
	}
	if r.Types.Kind(elem) == KParam || r.Types.Kind(elem) == KVar {
		return ""
	}
	if !r.SimdElem(elem) {
		return fmt.Sprintf("`%s` is not a lane type; use i8, i16, i32, i64, u8, u16, u32, u64, f32 or f64", r.TypeString(elem))
	}
	bits := int64(r.Types.Width(elem)) * n
	if bits != SimdVecBits {
		return fmt.Sprintf("it is %d bits, but a vector holds exactly %d", bits, SimdVecBits)
	}
	return ""
}

// checkSimdVectors rejects a std/simd call whose instantiated signature names
// a Vec that no 128-bit register can hold, before the backend has to.
func (c *checker) checkSimdVectors(n syntax.NodeID, sig, recv TypeID) {
	r := c.r
	var walk func(TypeID)
	walk = func(t TypeID) {
		if t == 0 {
			return
		}
		if why := r.SimdVecProblem(t); why != "" {
			c.errAt(n, cSimdShape, r.TypeString(t), why)
			return
		}
		for _, a := range r.Types.Node(t).Args {
			walk(a)
		}
	}
	walk(recv)
	walk(sig)
}

// simdCallKey names a bodiless std/simd function by the key the backends lower.
// It is set only for a call directly to std/simd.
func (c *checker) inStdSimd(fn EntityID) bool {
	e := c.r.Entity(fn)
	return e.Kind == EntFn && c.r.Packages[e.Pkg].Path == "std/simd"
}
