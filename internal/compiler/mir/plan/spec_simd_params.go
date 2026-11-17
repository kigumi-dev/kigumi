package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (s *specializer) simdShapeParams() map[sem.EntityID]map[sem.EntityID]bool {
	if s.simdArgs != nil {
		return s.simdArgs
	}
	r := s.p.R
	s.simdArgs = map[sem.EntityID]map[sem.EntityID]bool{}
	for _, f := range s.p.Funcs {
		if r.Specializable(f.Ent) || s.simdAsync(f.Ent) || f.Ent != 0 && r.Entity(f.Ent).Kind == sem.EntClosure {
			s.simdArgs[f.Ent] = map[sem.EntityID]bool{}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, f := range s.p.Funcs {
			params := s.simdArgs[f.Ent]
			if params == nil {
				continue
			}
			add := func(t sem.TypeID) {
				for _, p := range r.Types.Params(t) {
					if !params[p] {
						params[p], changed = true, true
					}
				}
			}
			for _, blk := range f.Blocks {
				for _, in := range blk.Insts {
					if in.Op == midir.OpClosure && r.Entity(in.Ent).Kind == sem.EntClosure {
						for p := range s.simdArgs[in.Ent] {
							if !params[p] {
								params[p], changed = true, true
							}
						}
						continue
					}
					if s.p.SimdCall(in) {
						add(f.Locals[in.Dst].Type)
						for _, a := range in.Args {
							add(f.Locals[a].Type)
						}
						continue
					}
					callee := s.simdArgs[in.Ent]
					if len(callee) == 0 {
						continue
					}
					call := s.callBinding(f, in)
					args, ok := s.p.bindTypeArgs(f, call, func(t sem.TypeID) sem.TypeID { return t })
					if !ok {
						continue
					}
					for i, p := range r.Fn(in.Ent).TypeParams {
						if callee[p] {
							add(args[i])
						}
					}
				}
			}
		}
	}
	return s.simdArgs
}
