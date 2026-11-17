package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
)

func (s *specializer) simdShapeArgs(gen sem.EntityID, args []sem.TypeID) bool {
	_, ok := s.simdArgsOnly(gen, args)
	return ok
}

func (s *specializer) simdArgsOnly(gen sem.EntityID, args []sem.TypeID) ([]sem.TypeID, bool) {
	r := s.p.R
	info := r.Fn(gen)
	shapeParams := s.simdShapeParams()[gen]
	if len(shapeParams) == 0 {
		return nil, false
	}
	out := make([]sem.TypeID, len(args))
	selected := false
	for i, p := range info.TypeParams {
		if !shapeParams[p] || r.IsLifetimeParam(p) {
			continue
		}
		if i >= len(args) || !r.ConcreteArg(args[i]) {
			return nil, false
		}
		out[i] = args[i]
		selected = true
	}
	return out, selected
}

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

func (s *specializer) reportSimdProblem(f *midir.Func, node syntax.NodeID, gen sem.EntityID, fn *midir.Func) bool {
	t, why, bad := s.simdProblem(gen, fn)
	if !bad {
		return false
	}
	s.report(diag.Error, f, node, "simd-shape", s.p.R.TypeString(t), why)
	return true
}
