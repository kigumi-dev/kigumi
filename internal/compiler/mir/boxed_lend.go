package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// A free generic function every caller reaches through an instance never runs its shared body.
func (p *Program) SharedUnreached(f *midir.Func, reached map[sem.EntityID]bool) bool {
	if f.Ent == 0 || reached[f.Ent] || p.R.Entity(f.Ent).Kind != sem.EntFn {
		return false
	}
	info := p.R.Fn(f.Ent)
	return len(info.TypeParams) > 0 && info.Recv == sem.RecvNone && info.Owner == 0 && f != p.Entry
}

func (p *Program) BoxedLend(f *midir.Func, in midir.Inst) sem.TypeID {
	if in.Op != midir.OpBorrow || in.Index != 0 || !p.R.LendSite(f.File, in.Node) {
		return 0
	}
	if t := f.Locals[in.Args[0]].Type; p.R.DirectAgg(t) && !p.nullaryDef(f, in.Args[0]) {
		return t
	}
	return 0
}

// A payload-free variant is one immortal every site shares, so borrowing it costs nothing.
func (p *Program) nullaryDef(f *midir.Func, l midir.LocalID) bool {
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() && in.Dst == l {
				return in.Op == midir.OpVariant && len(in.Args) == 0
			}
		}
	}
	return false
}
