package mir

func (f *Func) IsDirect(l LocalID) bool { return int(l) < len(f.Direct) && f.Direct[l] }

// framed reports a local kept in an async frame slot, which holds one pointer.
func framed(f *Func, l LocalID) bool { return int(l) < len(f.Slot) && f.Slot[l] >= 0 }

// In the boxed model, a write through one of these reaches the owner's
// storage, not a copy.
func readsForeign(op Opcode) bool {
	switch op {
	case OpCellGet, OpIndex, OpPayload, OpUnbox, OpGlobalGet, OpField, OpFieldMove:
		return true
	}
	return false
}

func foreignLocals(f *Func) []bool {
	out := make([]bool, len(f.Locals))
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !in.Op.Yields() || out[in.Dst] {
					continue
				}
				same := in.Op == OpMove || in.Op == OpAlias || (in.Op == OpShare && !in.Copy)
				if readsForeign(in.Op) || (same && out[in.Args[0]]) {
					out[in.Dst], changed = true, true
				}
			}
		}
	}
	return out
}

// classifyDirect answers from the local's type alone, less the pinned
// locals; PlanDirect writes every disagreement out as a crossing.
func (p *Program) classifyDirect(f *Func, pin []bool) []bool {
	out := make([]bool, len(f.Locals))
	if f.Async {
		return out
	}
	cand := make([]bool, len(f.Locals))
	for i, l := range f.Locals {
		cand[i] = p.R.DirectAgg(l.Type)
		out[i] = cand[i] && !pin[i]
	}
	for i, prm := range f.Params {
		out[prm] = f.DSig.Param(i) != 0
	}
	for i, use := range p.directUses(f, cand) {
		if !use {
			out[i] = false
		}
	}
	return out
}
