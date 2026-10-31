package mir

import "kigumi/internal/compiler/sem"

// resolveStdOps caches each call-shaped instruction's Prim/Prov on Inst, resolved from its Ent once.
func resolveStdOps(p *Program) {
	for _, f := range p.Funcs {
		for bi := range f.Blocks {
			insts := f.Blocks[bi].Insts
			for ii := range insts {
				if in := &insts[ii]; namesStdFn(p.R, in) {
					in.Prim, in.Prov, _ = ResolveStdOp(p.R, in.Ent)
				}
			}
		}
	}
}

func namesStdFn(r *sem.Result, in *Inst) bool {
	switch in.Op {
	case OpCall, OpClosure, OpFnItem, OpBind:
	default:
		return false
	}
	// OpClosure's Ent is an ordinary closure unless Kind is EntFn (a bound/async fn value).
	e := r.Entity(in.Ent)
	return e.Kind == sem.EntFn && e.Flags&sem.EfStd != 0
}

// stdOpOf is in.StdOp, also for an instruction resolveStdOps has not reached yet.
func stdOpOf(r *sem.Result, in *Inst) (StdOpId, bool) {
	if in.Prim != 0 || in.Prov != 0 || !namesStdFn(r, in) {
		return in.StdOp()
	}
	resolved := Inst{}
	resolved.Prim, resolved.Prov, _ = ResolveStdOp(r, in.Ent)
	return resolved.StdOp()
}
