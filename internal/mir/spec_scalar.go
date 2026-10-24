package mir

import "kigumi/internal/sem"

func (s *specializer) drain() {
	for len(s.work) > 0 {
		sp := s.work[0]
		s.work = s.work[1:]
		for _, f := range sp.funcs() {
			s.devirtualize(sp, f)
			s.scan(f, sp)
		}
	}
}

// Shared bodies name their aggregate instances first, so a scalar instance copies them rather than re-deciding.
func (s *specializer) scalarPhase() {
	s.scalars = true
	for _, f := range append([]*Func(nil), s.p.Funcs...) {
		s.scan(f, s.byFunc[f])
	}
	s.drain()
}

func (s *specializer) scalarArgs(args []sem.TypeID) bool {
	if !s.scalars {
		return false
	}
	for _, t := range args {
		if t != 0 && s.p.R.RegScalar(t) {
			return true
		}
	}
	return false
}

// A scalar instance never brings in an aggregate instance, so no chain grows from one; a held one has its own limit.
func (s *specializer) grows(ctx *spec, args []sem.TypeID) bool {
	return ctx != nil && ctx.held == nil && ctx.root.scalar && s.p.R.SpecializeArgs(args)
}

// A scalar instance gains by returning an aggregate unboxed or a type-parameter scalar in a register;
// a noalloc generic also takes its type-parameter scalars in registers.
func (s *specializer) scalarPays(sp *spec) (pays, inReg bool) {
	g := s.p.ByEnt[sp.gen]
	derived := !s.p.sameType(sp.fn.Ret, g.Ret)
	if derived && s.p.R.RegScalar(sp.fn.Ret) {
		return true, true
	}
	if s.p.R.Summary(g.Ent).Noalloc {
		for i, prm := range sp.fn.Params {
			t := sp.fn.Locals[prm].Type
			if i < len(g.Params) && s.p.R.RegScalar(t) && !s.p.sameType(t, g.Locals[g.Params[i]].Type) {
				return true, true
			}
		}
	}
	return derived && s.p.R.DirectAgg(sp.fn.Ret), false
}
