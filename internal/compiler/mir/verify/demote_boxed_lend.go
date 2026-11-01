package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// demoteBoxedLends tells sem about the lends planning boxed and returns the named callees the claims treat as reached.
func (p *claims) demoteBoxedLends() map[sem.EntityID]bool {
	reached := p.namedCallees()
	p.demoteLends(reached)
	return reached
}

func (p *claims) namedCallees() map[sem.EntityID]bool {
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

// An undeclared body is noalloc only while its lends stay direct; one that boxes allocates after all, which callers learn here.
// A witness bind that survives planning allocates, declared `noalloc` or not.
func (p *claims) demoteLends(reached map[sem.EntityID]bool) {
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

func (p *claims) keptWitnessBind(f *midir.Func, in midir.Inst) bool {
	return in.Op == midir.OpBind && len(in.Args) > 0 && p.R.LendSite(f.File, in.Node)
}
