package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
)

// A direct Option boxed again at its use adds an allocation the op summary does not charge.
func (p *Program) settlePrims(f *midir.Func, pin []bool) {
	for p.unmarkBoxedPrims(f) {
		f.Direct = p.classifyDirect(f, pin)
	}
}

func (p *Program) unmarkBoxedPrims(f *midir.Func) bool {
	origin := map[midir.LocalID][]*midir.Inst{}
	for bid := range f.Blocks {
		insts := f.Blocks[bid].Insts
		for i := range insts {
			if in := &insts[i]; in.Op == midir.OpCall && in.DPrim != opdef.DPNone && p.R.DirectAgg(f.Locals[in.Dst].Type) {
				origin[in.Dst] = append(origin[in.Dst], in)
			}
		}
	}
	if len(origin) == 0 {
		return false
	}
	bad := map[*midir.Inst]bool{}
	for changed := true; changed; {
		changed = false
		for bid := range f.Blocks {
			insts := f.Blocks[bid].Insts
			for i := range insts {
				changed = p.flowPrimOrigins(f, insts[i], origin, bad) || changed
			}
			if t := f.Blocks[bid].Term; t.Op == midir.TermReturn && len(origin[t.Args[0]]) > 0 {
				if cross, _, ok := p.returnCross(f, t.Args[0]); ok && cross == midir.OpBoxDirect {
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

func (p *Program) flowPrimOrigins(f *midir.Func, in midir.Inst, origin map[midir.LocalID][]*midir.Inst, bad map[*midir.Inst]bool) bool {
	changed := false
	for i, a := range in.Args {
		srcs := origin[a]
		if len(srcs) == 0 {
			continue
		}
		if cross, _, ok := p.argCross(f, in, i, a); ok && cross == midir.OpBoxDirect {
			changed = markBad(srcs, bad) || changed
		}
		switch in.Op {
		case midir.OpMove, midir.OpAlias, midir.OpCopy, midir.OpShare:
			changed = joinOrigins(origin, in.Dst, srcs) || changed
		}
	}
	if in.Op == midir.OpCall && in.DPrim != opdef.DPNone && len(origin[in.Dst]) > 0 {
		if cross, _, ok := p.dstCross(f, in); ok && cross == midir.OpBoxDirect {
			changed = markBad(origin[in.Dst], bad) || changed
		}
	}
	return changed
}

func markBad(srcs []*midir.Inst, bad map[*midir.Inst]bool) bool {
	changed := false
	for _, s := range srcs {
		if !bad[s] {
			bad[s], changed = true, true
		}
	}
	return changed
}

func joinOrigins(origin map[midir.LocalID][]*midir.Inst, dst midir.LocalID, srcs []*midir.Inst) bool {
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
