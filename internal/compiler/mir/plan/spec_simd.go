package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
)

// simdShapeArgs reports whether one of gen's const type arguments names part of
// a std/simd value's shape, and gen's shared body lowers a std/simd primitive
// on that value. Such a primitive expands at the call site into one 128-bit
// vector, so a Vec[T, N] with N still a parameter has no shape llgen can lower:
// the const arguments must be specialized into the body before plan-direct, the
// same way a type argument is. A body whose SIMD shapes are already concrete
// names no shape parameter and is left shared.
func (s *specializer) simdShapeArgs(gen sem.EntityID, args []sem.TypeID) bool {
	r := s.p.R
	info := r.Fn(gen)
	shapeParams := map[sem.EntityID]bool{}
	for i, t := range args {
		if t != 0 && i < len(info.TypeParams) && r.Types.IsConst(t) {
			shapeParams[info.TypeParams[i]] = true
		}
	}
	if len(shapeParams) == 0 {
		return false
	}
	names := func(t sem.TypeID) bool {
		for _, p := range r.Types.Params(t) {
			if shapeParams[p] {
				return true
			}
		}
		return false
	}
	if !s.bodyLowersSimd(gen) {
		return false
	}
	body := s.p.ByEnt[gen]
	for _, blk := range body.Blocks {
		for _, call := range blk.Insts {
			if !s.p.SimdCall(call) {
				continue
			}
			if names(body.Locals[call.Dst].Type) {
				return true
			}
			for _, a := range call.Args {
				if names(body.Locals[a].Type) {
					return true
				}
			}
		}
	}
	return false
}

// bodyLowersSimd caches whether a generic body calls a std/simd primitive at
// all, so the instantiation check costs one body scan per callee, not one per
// instance.
func (s *specializer) bodyLowersSimd(gen sem.EntityID) bool {
	if has, ok := s.simdBody[gen]; ok {
		return has
	}
	has := false
	if body := s.p.ByEnt[gen]; body != nil {
		for _, blk := range body.Blocks {
			for _, in := range blk.Insts {
				if s.p.SimdCall(in) {
					has = true
				}
			}
		}
	}
	s.simdBody[gen] = has
	return has
}

// simdProblem reports the first std/simd value an instance of gen would hold
// that no 128-bit vector can: the checker's E976 rule runs on a concrete call
// site, but a call inside a generic body names T or N, not the instantiation,
// so only the specialized body can be checked. The caller reports it at the
// call that supplied the arguments.
func (s *specializer) simdProblem(gen sem.EntityID, fn *midir.Func) (sem.TypeID, string, bool) {
	if !s.bodyLowersSimd(gen) {
		return 0, "", false
	}
	r := s.p.R
	bad := false
	var badType sem.TypeID
	why := ""
	check := func(t sem.TypeID) {
		if bad {
			return
		}
		if w := r.SimdVecProblem(t); w != "" {
			bad, badType, why = true, t, w
		}
	}
	for _, blk := range fn.Blocks {
		for _, call := range blk.Insts {
			if !s.p.SimdCall(call) {
				continue
			}
			check(fn.Locals[call.Dst].Type)
			for _, a := range call.Args {
				check(fn.Locals[a].Type)
			}
		}
	}
	return badType, why, bad
}

// reportSimdProblem records the diagnostic for an instance whose lane type or
// width the vector cannot hold, at the call that instantiated it.
func (s *specializer) reportSimdProblem(f *midir.Func, node syntax.NodeID, gen sem.EntityID, fn *midir.Func) bool {
	t, why, bad := s.simdProblem(gen, fn)
	if !bad {
		return false
	}
	s.report(diag.Error, f, node, "simd-shape", s.p.R.TypeString(t), why)
	return true
}
