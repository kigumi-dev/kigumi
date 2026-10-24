package mir

import "kigumi/internal/sem"

// markValueArgs records, with the types plan-rc planned under, how each value call passes its arguments: an erased `U`
// slot hands over a count, a `&T` slot lends. A specialised copy keeps the record, as it keeps the generic body's releases.
func (p *Program) markValueArgs(f *Func) {
	for b := range f.Blocks {
		for i := range f.Blocks[b].Insts {
			if in := &f.Blocks[b].Insts[i]; in.Op == OpCallValue && in.Ent == 0 {
				p.declareValueArgs(f, in)
			}
		}
	}
}

// settleValueArgs drops the arguments PlanDirect unboxed and records the value calls made since plan-rc.
func (p *Program) settleValueArgs(f *Func) {
	for b := range f.Blocks {
		for i := range f.Blocks[b].Insts {
			in := &f.Blocks[b].Insts[i]
			switch {
			case in.Op != OpCallValue:
				continue
			case in.Ent != 0:
				in.GiveArgs, in.LendArgs = 0, 0
				continue
			case !in.argsDeclared:
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

func (p *Program) declareValueArgs(f *Func, in *Inst) {
	in.GiveArgs, in.LendArgs, in.argsDeclared = 0, 0, true
	site := p.R.Types.Node(f.Locals[in.Args[0]].Type)
	for k := 1; k < len(in.Args) && k < 32; k++ {
		switch {
		case !p.consumes(f, *in, k):
			in.LendArgs |= 1 << (k - 1)
		case site.Kind != sem.KFn || k-1 >= len(site.Args) || len(p.R.Types.Params(site.Args[k-1])) > 0:
			in.GiveArgs |= 1 << (k - 1)
		}
	}
}
