package mir

import "kigumi/internal/sem"

func (s *specializer) instance(f *Func, ctx *spec, in Inst) ([]sem.TypeID, []sem.WitnessArg, bool) {
	r := s.p.R
	gen := in.Ent
	if s.p.ByEnt[gen] == nil || !r.Specializable(gen) || s.noSpec[gen] {
		return nil, nil, false
	}
	if _, minted := r.InstanceOf(gen); minted {
		return nil, nil, false
	}
	args, ok := s.p.bindTypeArgs(f, in, func(t sem.TypeID) sem.TypeID { return s.inCtx(ctx, t) })
	if !ok {
		return nil, nil, false
	}
	for _, t := range args {
		if t != 0 && !r.ConcreteArg(t) {
			return nil, nil, false
		}
	}
	if !r.SpecializeArgs(args) && !s.protoInstance(gen, args) && !s.scalarArgs(args) || s.grows(ctx, args) {
		return nil, nil, false
	}
	if in.Op == OpFnItem {
		// A function value reaches a scalar instance through an adapter that boxes the result again.
		return args, nil, r.SpecializeArgs(args) || s.protoInstance(gen, args)
	}
	return args, s.witnesses(ctx, gen, r.Files[f.File].Calls[in.Node]), true
}

func (p *Program) bindTypeArgs(f *Func, in Inst, subst func(sem.TypeID) sem.TypeID) ([]sem.TypeID, bool) {
	r := p.R
	gen := in.Ent
	info := r.Fn(gen)
	want := map[sem.EntityID]bool{}
	for _, tp := range info.TypeParams {
		want[tp] = !r.IsLifetimeParam(tp)
	}
	bound := map[sem.EntityID]sem.TypeID{}
	if in.Op == OpFnItem {
		if in.Async || info.Recv != sem.RecvNone || len(info.Witnesses) > 0 {
			return nil, false
		}
		if !r.Types.BindParams(info.Sig, f.Locals[in.Dst].Type, want, bound) {
			return nil, false
		}
		return p.boundArgs(gen, want, bound)
	}
	call := r.Files[f.File].Calls[in.Node]
	if call.Callee == gen {
		own := p.ownParams(gen)
		for k, tp := range own {
			if k < len(call.Inst) && want[tp] && len(call.Inst) == len(own) {
				bound[tp] = subst(call.Inst[k])
			}
		}
	}
	decl := p.declaredArgs(gen)
	if decl == nil || len(in.Args) < len(decl) {
		return nil, false
	}
	for i, d := range decl {
		at := f.Locals[in.Args[i]].Type
		if i == 0 && info.Recv != sem.RecvNone && r.Types.Kind(at) == sem.KRef {
			at = r.Types.Node(at).Elem
		}
		if !r.Types.BindParams(d, at, want, bound) {
			return nil, false
		}
	}
	if !r.Types.BindParams(r.Types.Node(info.Sig).Elem, f.Locals[in.Dst].Type, want, bound) {
		return nil, false
	}
	return p.boundArgs(gen, want, bound)
}

func (p *Program) boundArgs(gen sem.EntityID, want map[sem.EntityID]bool, bound map[sem.EntityID]sem.TypeID) ([]sem.TypeID, bool) {
	r := p.R
	tps := r.Fn(gen).TypeParams
	args := make([]sem.TypeID, len(tps))
	for i, tp := range tps {
		if !want[tp] {
			continue
		}
		t, ok := bound[tp]
		if !ok {
			return nil, false
		}
		args[i] = t
	}
	return args, true
}

// As in instantiateFn, a method's leading owner parameters come from the receiver.
func (p *Program) ownParams(gen sem.EntityID) []sem.EntityID {
	tps := p.R.Fn(gen).TypeParams
	owner := p.R.OwnerParams(gen)
	if p.R.Fn(gen).Recv != sem.RecvNone && len(owner) > 0 && len(tps) >= len(owner) && tps[0] == owner[0] {
		return tps[len(owner):]
	}
	return tps
}

// A variadic callee's packed argument carries no instantiated type.
func (p *Program) declaredArgs(gen sem.EntityID) []sem.TypeID {
	r := p.R
	info := r.Fn(gen)
	out := []sem.TypeID{}
	if info.Recv != sem.RecvNone {
		out = append(out, r.Entity(info.SelfParam).Type)
	}
	for _, prm := range info.Params {
		if r.Entity(prm).Flags&sem.EfVariadic != 0 {
			return nil
		}
		out = append(out, r.Entity(prm).Type)
	}
	return out
}

func (s *specializer) inCtx(ctx *spec, t sem.TypeID) sem.TypeID {
	if ctx == nil {
		return t
	}
	return s.p.R.Types.Subst(t, ctx.sub)
}

func (s *specializer) witnesses(ctx *spec, gen sem.EntityID, call sem.CallInfo) []sem.WitnessArg {
	slots := s.p.R.Fn(gen).Witnesses
	if len(slots) == 0 || call.Callee != gen || len(call.Passes) != len(slots) {
		return nil
	}
	out := make([]sem.WitnessArg, len(slots))
	for i, w := range call.Passes {
		sw, ok := s.static(ctx, w)
		if !ok {
			return nil
		}
		out[i] = sw
	}
	return out
}

func (s *specializer) static(ctx *spec, w sem.WitnessArg) (sem.WitnessArg, bool) {
	switch w.Kind {
	case sem.WitnessFn:
		return w, true
	case sem.WitnessBind:
		out := w
		out.Args = make([]sem.WitnessArg, len(w.Args))
		for i, a := range w.Args {
			sa, ok := s.static(ctx, a)
			if !ok {
				return sem.WitnessArg{}, false
			}
			out.Args[i] = sa
		}
		return out, true
	case sem.WitnessLocal:
		if ctx == nil || ctx.wit == nil {
			return sem.WitnessArg{}, false
		}
		for k, slot := range s.p.R.Fn(ctx.gen).Witnesses {
			if slot.Local == w.Ent {
				return ctx.wit[k], true
			}
		}
	}
	return sem.WitnessArg{}, false
}

// The body becomes the direct comparison or hash, so scalar arguments are specialized too.
func (s *specializer) protoInstance(gen sem.EntityID, args []sem.TypeID) bool {
	r := s.p.R
	kind, self := r.ProtoKind(gen), r.ProtoSelf(gen)
	return kind != "" && self != 0 && r.ProtocolDirect(r.Types.Subst(self, r.InstanceSubst(gen, args)), kind)
}
