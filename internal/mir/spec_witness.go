package mir

import "kigumi/internal/sem"

// Conformance fixes the implementation's receiver kind and parameter types, so plan-rc's plan still holds.
func (s *specializer) devirtualize(sp *spec, f *Func) {
	if sp.wit == nil {
		return
	}
	slots := s.p.R.Fn(sp.gen).Witnesses
	defs := singleDefs(f)
	for bid := range f.Blocks {
		insts := f.Blocks[bid].Insts
		for i := range insts {
			in := &insts[i]
			if in.Op == OpBuiltin && in.Str == builtinCmpUnordered {
				s.foldUnordered(sp, f, defs, slots, in)
				continue
			}
			if in.Op != OpCallValue || in.Ent == 0 || len(in.Args) == 0 {
				continue
			}
			k := witnessSlotOf(f, defs, in.Args[0], slots)
			if k < 0 || sp.wit[k].Kind != sem.WitnessFn || !s.directImpl(sp.wit[k].Ent, *in) {
				continue
			}
			in.Op, in.Ent, in.Args = OpCall, sp.wit[k].Ent, in.Args[1:]
		}
	}
	foldCompareGuards(f, defs)
}

func (s *specializer) directImpl(m sem.EntityID, in Inst) bool {
	r := s.p.R
	if s.p.ByEnt[m] == nil || r.Entity(m).Kind != sem.EntFn {
		return false
	}
	info, req := r.Fn(m), r.Fn(in.Ent)
	if info.Declared&sem.EffAsync != 0 || info.Recv != req.Recv {
		return false
	}
	if len(info.Witnesses) > 0 || len(info.ConstParams) > 0 || len(info.CtxRequires) > 0 {
		return false
	}
	arity := len(info.Params)
	if info.Recv != sem.RecvNone {
		arity++
	}
	return arity == len(in.Args)-1
}

func witnessSlotOf(f *Func, defs map[LocalID]*Inst, l LocalID, slots []sem.WitnessSlot) int {
	for steps := 0; steps < len(f.Locals); steps++ {
		if isParam(f, l) {
			for k, slot := range slots {
				if f.Locals[l].Ent == slot.Local {
					return k
				}
			}
			return -1
		}
		d := defs[l]
		if d == nil {
			return -1
		}
		switch d.Op {
		case OpCopy, OpAlias, OpShare, OpMove:
			l = d.Args[0]
		default:
			return -1
		}
	}
	return -1
}

func singleDefs(f *Func) map[LocalID]*Inst {
	out := map[LocalID]*Inst{}
	count := map[LocalID]int{}
	for bid := range f.Blocks {
		insts := f.Blocks[bid].Insts
		for i := range insts {
			if insts[i].Op.Yields() {
				count[insts[i].Dst]++
				out[insts[i].Dst] = &insts[i]
			}
		}
	}
	for l, n := range count {
		if n != 1 {
			delete(out, l)
		}
	}
	return out
}
