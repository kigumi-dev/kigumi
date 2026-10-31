package mir

import (
	opdef "kigumi/internal/compiler/mir/ops"
)

// A direct Option boxed again at its use adds an allocation the op summary does not charge.
func (p *Program) settlePrims(f *Func, pin []bool) {
	for p.unmarkBoxedPrims(f) {
		f.Direct = p.classifyDirect(f, pin)
	}
}

func (p *Program) unmarkBoxedPrims(f *Func) bool {
	origin := map[LocalID][]*Inst{}
	for bid := range f.Blocks {
		insts := f.Blocks[bid].Insts
		for i := range insts {
			if in := &insts[i]; in.Op == OpCall && in.DPrim != opdef.DPNone && p.R.DirectAgg(f.Locals[in.Dst].Type) {
				origin[in.Dst] = append(origin[in.Dst], in)
			}
		}
	}
	if len(origin) == 0 {
		return false
	}
	bad := map[*Inst]bool{}
	for changed := true; changed; {
		changed = false
		for bid := range f.Blocks {
			insts := f.Blocks[bid].Insts
			for i := range insts {
				changed = p.flowPrimOrigins(f, insts[i], origin, bad) || changed
			}
			if t := f.Blocks[bid].Term; t.Op == TermReturn && len(origin[t.Args[0]]) > 0 {
				if cross, _, ok := p.returnCross(f, t.Args[0]); ok && cross == OpBoxDirect {
					changed = markBad(origin[t.Args[0]], bad) || changed
				}
			}
		}
	}
	for in := range bad {
		in.DPrim = opdef.DPNone
	}
	return len(bad) > 0
}

func (p *Program) flowPrimOrigins(f *Func, in Inst, origin map[LocalID][]*Inst, bad map[*Inst]bool) bool {
	changed := false
	for i, a := range in.Args {
		srcs := origin[a]
		if len(srcs) == 0 {
			continue
		}
		if cross, _, ok := p.argCross(f, in, i, a); ok && cross == OpBoxDirect {
			changed = markBad(srcs, bad) || changed
		}
		switch in.Op {
		case OpMove, OpAlias, OpCopy, OpShare:
			changed = joinOrigins(origin, in.Dst, srcs) || changed
		}
	}
	if in.Op == OpCall && in.DPrim != opdef.DPNone && len(origin[in.Dst]) > 0 {
		if cross, _, ok := p.dstCross(f, in); ok && cross == OpBoxDirect {
			changed = markBad(origin[in.Dst], bad) || changed
		}
	}
	return changed
}

func markBad(srcs []*Inst, bad map[*Inst]bool) bool {
	changed := false
	for _, s := range srcs {
		if !bad[s] {
			bad[s], changed = true, true
		}
	}
	return changed
}

func joinOrigins(origin map[LocalID][]*Inst, dst LocalID, srcs []*Inst) bool {
	changed := false
	for _, s := range srcs {
		have := false
		for _, o := range origin[dst] {
			have = have || o == s
		}
		if !have {
			origin[dst], changed = append(origin[dst], s), true
		}
	}
	return changed
}
