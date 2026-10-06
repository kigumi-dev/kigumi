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

// A local Borrowed by one of these can still be unboxed: copying it owns no
// reference to protect.
func copyDef(op Opcode) bool {
	switch op {
	case OpField, OpFieldMove, OpCopy, OpAlias, OpMove, OpShare, OpRecord:
		return true
	}
	return false
}

func copyDefOnly(f *Func) []bool {
	out := make([]bool, len(f.Locals))
	seen := make([]bool, len(f.Locals))
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() {
				continue
			}
			out[in.Dst] = copyDef(in.Op) && (!seen[in.Dst] || out[in.Dst])
			seen[in.Dst] = true
		}
	}
	return out
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

// An async frame slot holds one pointer, so async bodies are excluded.
func (p *Program) classifyDirect(f *Func) []bool {
	out := make([]bool, len(f.Locals))
	if f.Async {
		return out
	}
	isParam := make([]bool, len(f.Locals))
	for _, prm := range f.Params {
		isParam[prm] = true
	}
	cand := make([]bool, len(f.Locals))
	for i, l := range f.Locals {
		cand[i] = p.R.DirectAgg(l.Type)
	}
	copyDefined := copyDefOnly(f)
	for i, l := range f.Locals {
		if isParam[i] || l.Cell || framed(f, LocalID(i)) || (l.Borrowed && !copyDefined[i]) {
			continue
		}
		out[i] = cand[i]
	}
	for i, prm := range f.Params {
		out[prm] = f.DSig.Param(i) != 0
	}
	foreign := foreignLocals(f)
	p.settleDirect(f, out, cand, foreign)
	for i, use := range p.directUses(f, cand) {
		if !use {
			out[i] = false
		}
	}
	p.settleDirect(f, out, cand, foreign)
	return out
}

func (p *Program) settleDirect(f *Func, out, cand, foreign []bool) {
	for changed := true; changed; {
		changed = false
		drop := func(l LocalID) {
			if out[l] {
				out[l], changed = false, true
			}
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				classifyDirectUses(in, out, cand, foreign, drop)
			}
		}
	}
}

// A borrow needs an address and a write through an alias must land, so
// neither keeps its local unboxed.
func classifyDirectUses(in Inst, out, cand, foreign []bool, drop func(LocalID)) {
	switch in.Op {
	case OpAsm, OpCallC, OpShell:
		for _, a := range in.Args {
			drop(a)
		}
	case OpBorrow, OpNewCell:
		// OpNewCell's result has the content's type but holds a cell object.
		drop(in.Args[0])
		drop(in.Dst)
	case OpCellGet, OpIndex, OpPayload, OpUnbox, OpGlobalGet:
		drop(in.Dst)
	case OpSetField:
		if foreign[in.Args[0]] {
			drop(in.Args[0])
		}
	case OpField, OpFieldMove:
		if cand[in.Dst] {
			agree(in, out, drop)
		}
	case OpMove, OpCopy, OpShare, OpAlias:
		if cand[in.Dst] || cand[in.Args[0]] {
			agree(in, out, drop)
		}
	}
}

// The boxed model hands back the same object; an unboxed aggregate, a copy.
func agree(in Inst, out []bool, drop func(LocalID)) {
	if out[in.Args[0]] != out[in.Dst] {
		drop(in.Args[0])
		drop(in.Dst)
	}
}
