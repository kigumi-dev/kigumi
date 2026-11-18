package plan

import (
	"slices"

	"kigumi/internal/compiler/mir/instance"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// valueArgs binds the type arguments of a function value no call binds: a method value from its receiver, a fn
// item or bind with hidden parameters from its own type.
func (p *planner) valueArgs(f *midir.Func, in midir.Inst, subst func(sem.TypeID) sem.TypeID) ([]sem.TypeID, bool) {
	r := p.R
	gen := in.Ent
	if gen == 0 || r.Entity(gen).Kind != sem.EntFn || r.Entity(r.Entity(gen).Parent).Kind == sem.EntInterface {
		return nil, false
	}
	info := r.Fn(gen)
	bound := map[sem.EntityID]sem.TypeID{}
	switch {
	case in.Op == midir.OpClosure && (in.Form() == midir.FormMethod || in.Form() == midir.FormApplied):
		recv := subst(f.Locals[in.Args[len(in.Args)-1]].Type)
		if r.Types.Kind(recv) == sem.KRef {
			recv = r.Types.Node(recv).Elem
		}
		b, ok := r.ReceiverArgs(gen, recv)
		if !ok {
			return nil, false
		}
		bound = b
		want := map[sem.EntityID]bool{}
		for _, tp := range p.ownParams(gen) {
			want[tp] = !r.IsLifetimeParam(tp)
		}
		if len(want) > 0 && !p.itemArgs(f, in, want, bound, subst) {
			return nil, false
		}
	case in.Op == midir.OpBind || in.Op == midir.OpFnItem && (info.Recv != sem.RecvNone || len(info.Witnesses) > 0):
		want := map[sem.EntityID]bool{}
		for _, p := range info.TypeParams {
			want[p] = !r.IsLifetimeParam(p)
		}
		if !p.itemArgs(f, in, want, bound, subst) {
			return nil, false
		}
	default:
		return nil, false
	}
	args := make([]sem.TypeID, len(info.TypeParams))
	for i, p := range info.TypeParams {
		if !r.IsLifetimeParam(p) {
			args[i] = bound[p]
		}
	}
	return args, true
}

// specializeValue points a function value whose body cannot run erased at the body for its instantiation; an
// applied method value's bind goes with it.
func (s *specializer) specializeValue(f *midir.Func, ctx *spec, bid, idx int) bool {
	in := f.Blocks[bid].Insts[idx]
	if _, minted := s.p.R.InstanceOf(in.Ent); minted || len(s.shapeParams()[in.Ent]) == 0 {
		return false
	}
	args, ok := s.p.valueArgs(f, in, func(t sem.TypeID) sem.TypeID { return s.inCtx(ctx, t) })
	if !ok {
		return false
	}
	shapeArgs, ok := s.shapeArgsOnly(in.Ent, args)
	if !ok {
		return false
	}
	sp := s.get(f, ctx, in, s.p.R.EraseLifetimes(shapeArgs), nil)
	if sp == nil {
		return false
	}
	if in.Op == midir.OpClosure && in.Form() == midir.FormApplied {
		if pos, ok := s.defOf(f, in.Args[0]); ok && f.Blocks[pos.blk].Insts[pos.idx].Ent == in.Ent {
			s.rename(f, pos.blk, pos.idx, sp)
		}
	}
	s.rename(f, bid, idx, sp)
	return true
}

func (s *specializer) mentionsOwn(gen sem.EntityID, t sem.TypeID) bool {
	params := s.p.R.Fn(gen).TypeParams
	for _, p := range s.p.R.Types.Params(t) {
		if slices.Contains(params, p) {
			return true
		}
	}
	return false
}

// rename points the instruction at (bid, idx) at sp's body. A fn item typed through the generic's own parameters,
// as an async callee or value is, takes the instance's types with it.
func (s *specializer) rename(f *midir.Func, bid, idx int, sp *spec) {
	in := &f.Blocks[bid].Insts[idx]
	site := &callSite{f: f, blk: bid, idx: idx}
	edit := siteEdit{f: f, blk: bid, idx: idx, was: in.Ent}
	if t := f.Locals[in.Dst].Type; in.Op == midir.OpFnItem && s.mentionsOwn(in.Ent, t) {
		site.wasType, edit.wasType = t, t
		f.Locals[in.Dst].Type = s.p.R.Types.Subst(t, s.p.R.InstanceSubst(sp.gen, sp.args))
	}
	sp.sites = append(sp.sites, site)
	s.edits = append(s.edits, edit)
	in.Ent = sp.ent
}

// itemArgs binds a function value's type arguments: the instantiation sem recorded for the item, which binds a
// parameter its type does not spell, then the value's own type when that is concrete.
func (p *planner) itemArgs(f *midir.Func, in midir.Inst, want map[sem.EntityID]bool, bound map[sem.EntityID]sem.TypeID, subst func(sem.TypeID) sem.TypeID) bool {
	r := p.R
	gen := in.Ent
	if inst, ok := r.Files[f.File].ItemInst[in.Node]; ok {
		own := p.ownParams(gen)
		for k, tp := range own {
			if k < len(inst) && want[tp] && len(inst) == len(own) {
				bound[tp] = subst(inst[k])
			}
		}
	}
	for tp, w := range want {
		if _, ok := bound[tp]; w && !ok {
			if t := subst(f.Locals[in.Dst].Type); instance.Concrete(r, t) {
				return r.Types.BindParams(r.Fn(gen).Sig, t, want, bound)
			}
			return true
		}
	}
	return true
}
