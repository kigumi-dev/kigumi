package mir

import "kigumi/internal/sem"

// ents renames the closures g builds to their copies.
func (p *Program) cloneFor(g *Func, ent sem.EntityID, name string, subst map[sem.EntityID]sem.TypeID, ents map[sem.EntityID]sem.EntityID) *Func {
	ty := func(t sem.TypeID) sem.TypeID { return p.R.Types.Subst(t, subst) }
	f := &Func{Name: name, Ent: ent, File: g.File, Ret: ty(g.Ret), ImplicitOk: g.ImplicitOk, rcBase: g.rcBase}
	f.Params = append([]LocalID(nil), g.Params...)
	f.PanicDrop = append([]LocalID(nil), g.PanicDrop...)
	f.Locals = make([]Local, len(g.Locals))
	for i, l := range g.Locals {
		l.Type = ty(l.Type)
		f.Locals[i] = l
	}
	f.Blocks = make([]Block, len(g.Blocks))
	for bid, blk := range g.Blocks {
		insts := make([]Inst, len(blk.Insts))
		for i, in := range blk.Insts {
			in.Args = append([]LocalID(nil), in.Args...)
			in.Type = ty(in.Type)
			if c, ok := ents[in.Ent]; ok && in.Op == OpClosure {
				in.Ent = c
			}
			insts[i] = in
		}
		term := Term{Op: blk.Term.Op, Args: append([]LocalID(nil), blk.Term.Args...), Targets: append([]BlockID(nil), blk.Term.Targets...)}
		f.Blocks[bid] = Block{Insts: insts, Term: term}
	}
	return f
}

func (s *specializer) genericClosure(t sem.TypeID, seen map[sem.TypeID]bool) bool {
	if t == 0 || seen[t] {
		return false
	}
	seen[t] = true
	tt := s.p.R.Types
	n := tt.Node(t)
	if n.Kind == sem.KClosure {
		if len(tt.Params(s.p.R.Closure(n.Ent).Sig)) > 0 {
			return true
		}
		for _, c := range s.p.R.AggFields(t) {
			if len(tt.Params(c)) > 0 {
				return true
			}
		}
		return false
	}
	if s.genericClosure(n.Elem, seen) {
		return true
	}
	for _, a := range n.Args {
		if s.genericClosure(a, seen) {
			return true
		}
	}
	return false
}
