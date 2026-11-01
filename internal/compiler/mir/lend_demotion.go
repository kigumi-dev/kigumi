package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *Program) namedCallees() map[sem.EntityID]bool {
	out := map[sem.EntityID]bool{}
	for _, f := range p.AllFuncs() {
		if f.Wraps != nil {
			continue
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				switch in.Op {
				case midir.OpCall, midir.OpFnItem, midir.OpBind, midir.OpClosure:
					out[in.Ent] = true
				}
			}
		}
	}
	return out
}

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

// DemoteBoxedLends tells sem about the lends planning boxed; the claim check reads sem after it and takes the calls it returns.
func DemoteBoxedLends(p *Program) map[sem.EntityID]bool {
	reached := p.namedCallees()
	p.demoteLends(reached)
	return reached
}

// An undeclared body is noalloc only while its lends stay direct; one that boxes allocates after all, which callers learn here.
// A witness bind that survives planning allocates, declared `noalloc` or not.
func (p *Program) demoteLends(reached map[sem.EntityID]bool) {
	for _, f := range p.AllFuncs() {
		if f.Ent == 0 || p.SharedUnreached(f, reached) {
			continue
		}
		declared := p.DeclaredNoalloc(f)
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !declared && p.BoxedLend(f, in) != 0 || p.keptWitnessBind(f, in) {
					p.R.DemoteBoxedLend(f.File, in.Node)
				}
			}
		}
	}
	p.R.SettleLendDemotions()
}

func (p *Program) keptWitnessBind(f *midir.Func, in midir.Inst) bool {
	return in.Op == midir.OpBind && len(in.Args) > 0 && p.R.LendSite(f.File, in.Node)
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
