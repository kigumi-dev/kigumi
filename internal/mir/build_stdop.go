package mir

import "kigumi/internal/sem"

// resolveStdOps caches each call-shaped instruction's Prim/Prov on Inst, resolved from its Ent once.
func resolveStdOps(p *Program) {
	for _, f := range p.Funcs {
		for bi := range f.Blocks {
			insts := f.Blocks[bi].Insts
			for ii := range insts {
				in := &insts[ii]
				switch in.Op {
				case OpCall, OpClosure, OpFnItem, OpBind:
				default:
					continue
				}
				// OpClosure's Ent is an ordinary closure unless Kind is EntFn (a bound/async fn value).
				e := p.R.Entity(in.Ent)
				if e.Kind != sem.EntFn || e.Flags&sem.EfStd == 0 {
					continue
				}
				in.Prim, in.Prov, _ = ResolveStdOp(p.R, in.Ent)
			}
		}
	}
}
