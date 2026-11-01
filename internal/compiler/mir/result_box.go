package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *Program) BoxedResult(f *midir.Func, in midir.Inst) sem.TypeID {
	switch in.Op {
	case midir.OpCallValue:
		if f.IsDirect(in.Args[0]) {
			return 0
		}
	case midir.OpCall:
		if p.R.Entity(p.R.Entity(in.Ent).Parent).Kind != sem.EntInterface {
			return 0
		}
	case midir.OpFnItem, midir.OpBind, midir.OpClosure:
		if g := p.ByEnt[in.Ent]; g != nil && !in.Async && (in.Op != midir.OpClosure || in.Str == "method") {
			return p.entryBox(g)
		}
		return 0
	default:
		return 0
	}
	if t := f.Locals[in.Dst].Type; p.R.BoxAllocates(t) {
		return t
	}
	return 0
}

func (p *Program) ResultBoxed(f *midir.Func, in midir.Inst) bool {
	switch in.Op {
	case midir.OpCallValue:
		return !f.IsDirect(in.Args[0])
	case midir.OpCall:
		return p.R.Entity(p.R.Entity(in.Ent).Parent).Kind == sem.EntInterface
	}
	return false
}

// entryBox is the type g's entry allocates a box for, 0 when it allocates none.
func (p *Program) entryBox(g *midir.Func) sem.TypeID {
	if g.Entry == nil {
		return 0
	}
	for _, blk := range g.Entry.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpBoxDirect && !in.Fixed && p.R.BoxAllocates(in.Type) {
				return in.Type
			}
		}
	}
	return 0
}
