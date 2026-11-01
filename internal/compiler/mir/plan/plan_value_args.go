package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// markValueArgs records, with the types plan-rc planned under, how each value call passes its arguments: an erased `U`
// slot hands over a count, a `&T` slot lends. A specialised copy keeps the record, as it keeps the generic body's releases.
func (p *planner) markValueArgs(f *midir.Func) {
	for b := range f.Blocks {
		for i := range f.Blocks[b].Insts {
			if in := &f.Blocks[b].Insts[i]; in.Op == midir.OpCallValue && in.Ent == 0 {
				p.declareValueArgs(f, in)
			}
		}
	}
}

// settleValueArgs drops the arguments PlanDirect unboxed and records the value calls made since plan-rc.
func (p *planner) settleValueArgs(f *midir.Func) {
	for b := range f.Blocks {
		for i := range f.Blocks[b].Insts {
			in := &f.Blocks[b].Insts[i]
			switch {
			case in.Op != midir.OpCallValue:
				continue
			case in.Ent != 0:
				in.GiveArgs, in.LendArgs = 0, 0
				continue
			case !in.ArgsDeclared:
				p.declareValueArgs(f, in)
			}
			for k := 1; k < len(in.Args) && k < 32; k++ {
				if f.IsDirect(in.Args[k]) {
					in.GiveArgs &^= 1 << (k - 1)
					in.LendArgs &^= 1 << (k - 1)
				}
			}
		}
	}
}

func (p *planner) declareValueArgs(f *midir.Func, in *midir.Inst) {
	in.GiveArgs, in.LendArgs, in.ArgsDeclared = 0, 0, true
	site := p.R.Types.Node(f.Locals[in.Args[0]].Type)
	for k := 1; k < len(in.Args) && k < 32; k++ {
		switch {
		case !p.Consumes(f, *in, k):
			in.LendArgs |= 1 << (k - 1)
		case site.Kind != sem.KFn || k-1 >= len(site.Args) || len(p.R.Types.Params(site.Args[k-1])) > 0:
			in.GiveArgs |= 1 << (k - 1)
		}
	}
}
