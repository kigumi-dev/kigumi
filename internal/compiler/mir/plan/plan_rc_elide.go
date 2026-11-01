package plan

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// ElideRC drops counts RC left that no observer can tell from their absence.
func ElideRC(p *mir.Program) error { return (&planner{Program: p}).elideRC() }

func (p *planner) elideRC() error {
	for _, f := range p.AllFuncs() {
		for b := range f.Blocks {
			f.Blocks[b].Insts = p.elideBlock(f, f.Blocks[b].Insts)
		}
	}
	return nil
}

// Void and Bool are the runtime's immortal singletons, so a count on one changes nothing.
func immortalLocal(f *midir.Func, l midir.LocalID) bool {
	t := f.Locals[l].Type
	return t == sem.TyUnit || t == sem.TyBool
}

func (p *planner) elideBlock(f *midir.Func, in []midir.Inst) []midir.Inst {
	out := make([]midir.Inst, 0, len(in))
	for k := 0; k < len(in); k++ {
		x := in[k]
		switch {
		case x.Op == midir.OpRelease && immortalLocal(f, x.Args[0]):
			continue
		case x.Op == midir.OpShare && immortalLocal(f, x.Args[0]):
			x = midir.Inst{Op: midir.OpAlias, Dst: x.Dst, Args: x.Args, Type: x.Type, Node: x.Node}
		case x.Op == midir.OpShare:
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

// elideShare cancels a retain the source's or its own release undoes, or hands the source's count to the
// new reference when the source dies right after it. Only drops of other locals may lie between.
func (p *planner) elideShare(f *midir.Func, run []midir.Inst) (keep []midir.Inst, gone int, ok bool) {
	share := run[0]
	if len(share.Args) != 1 || share.Copy && !ownquery.RetainOnly(p.R, f.Locals[share.Args[0]].Type) {
		return nil, 0, false
	}
	src, dst := share.Args[0], share.Dst
	if f.IsDirect(src) || f.IsDirect(dst) || src == dst {
		return nil, 0, false
	}
	for j := 1; j < len(run); j++ {
		next := run[j]
		if (next.Op != midir.OpRelease && next.Op != midir.OpDrop) || len(next.Args) != 1 {
			return nil, 0, false
		}
		switch rel := next.Args[0]; {
		case rel == dst && next.Op == midir.OpRelease && f.Locals[dst].Ent == 0 && !f.Locals[dst].Cell:
			return run[1:j], j, true
		case rel == src && p.diesAt(f, src, next):
			move := midir.Inst{Op: midir.OpMove, Dst: dst, Args: []midir.LocalID{src}, Type: share.Type, Node: share.Node}
			return append([]midir.Inst{move}, run[1:j]...), j, true
		case rel == src || rel == dst:
			return nil, 0, false
		}
	}
	return nil, 0, false
}

// diesAt reports that next ends the life of src, a local this body owns the count of.
func (p *planner) diesAt(f *midir.Func, src midir.LocalID, next midir.Inst) bool {
	l := f.Locals[src]
	if l.Cell {
		return false
	}
	if next.Op == midir.OpRelease {
		return l.Ent == 0
	}
	return l.Ent != 0 && !l.Borrowed
}
