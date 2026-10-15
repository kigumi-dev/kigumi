package mir

import "kigumi/internal/sem"

// ElideRC drops counts PlanRC left that no observer can tell from their absence.
func ElideRC(p *Program) error {
	for _, f := range p.allFuncs() {
		for b := range f.Blocks {
			f.Blocks[b].Insts = p.elideBlock(f, f.Blocks[b].Insts)
		}
	}
	return nil
}

// Unit and Bool are the runtime's immortal singletons, so a count on one changes nothing.
func immortalLocal(f *Func, l LocalID) bool {
	t := f.Locals[l].Type
	return t == sem.TyUnit || t == sem.TyBool
}

func (p *Program) elideBlock(f *Func, in []Inst) []Inst {
	out := make([]Inst, 0, len(in))
	for k := 0; k < len(in); k++ {
		x := in[k]
		switch {
		case x.Op == OpRelease && immortalLocal(f, x.Args[0]):
			continue
		case x.Op == OpShare && immortalLocal(f, x.Args[0]):
			x = Inst{Op: OpAlias, Dst: x.Dst, Args: x.Args, Type: x.Type, Node: x.Node}
		case x.Op == OpShare:
			if keep, gone, ok := p.elideShare(f, in[k:]); ok {
				out = append(out, keep...)
				k += gone
				continue
			}
		}
		out = append(out, x)
	}
	return out
}

// A copy of one of these is a retain: the value is immutable, so no one can tell the two apart.
func (p *Program) retainOnly(t sem.TypeID) bool {
	return p.R.Types.IsNumeric(t) || t == sem.TyBool || t == sem.TyChar || t == sem.TyString || t == sem.TyUnit
}

// elideShare cancels a retain the source's or its own release undoes, or hands the source's count to the
// new reference when the source dies right after it. Only drops of other locals may lie between.
func (p *Program) elideShare(f *Func, run []Inst) (keep []Inst, gone int, ok bool) {
	share := run[0]
	if len(share.Args) != 1 || share.Copy && !p.retainOnly(f.Locals[share.Args[0]].Type) {
		return nil, 0, false
	}
	src, dst := share.Args[0], share.Dst
	if f.IsDirect(src) || f.IsDirect(dst) || src == dst {
		return nil, 0, false
	}
	for j := 1; j < len(run); j++ {
		next := run[j]
		if (next.Op != OpRelease && next.Op != OpDrop) || len(next.Args) != 1 {
			return nil, 0, false
		}
		switch rel := next.Args[0]; {
		case rel == dst && next.Op == OpRelease && f.Locals[dst].Ent == 0 && !f.Locals[dst].Cell:
			return run[1:j], j, true
		case rel == src && p.diesAt(f, src, next):
			move := Inst{Op: OpMove, Dst: dst, Args: []LocalID{src}, Type: share.Type, Node: share.Node}
			return append([]Inst{move}, run[1:j]...), j, true
		case rel == src || rel == dst:
			return nil, 0, false
		}
	}
	return nil, 0, false
}

// diesAt reports that next ends the life of src, a local this body owns the count of.
func (p *Program) diesAt(f *Func, src LocalID, next Inst) bool {
	l := f.Locals[src]
	if l.Cell {
		return false
	}
	if next.Op == OpRelease {
		return l.Ent == 0
	}
	return l.Ent != 0 && !l.Borrowed
}
