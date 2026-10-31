package mir

import (
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

func (p *verifier) verifyStdOpTags(f *Func) *Error {
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Prim != 0 || in.Prov != 0 {
				prim, prov := p.resolvedTags(in)
				if in.Prim != prim || in.Prov != prov {
					return fail("stdop", BlockID(bid), i, "%s carries runtime op tags %d/%d, which its entity does not resolve to", in.Op, in.Prim, in.Prov)
				}
			}
			if in.DPrim != opdef.DPNone && (in.Op != OpCall || in.DPrim != p.directPrimOf(f, in)) {
				return fail("stdop", BlockID(bid), i, "%s carries direct result tag %d, which it does not take", in.Op, in.DPrim)
			}
		}
	}
	return nil
}

func (p *verifier) resolvedTags(in Inst) (opdef.PrimitiveId, opdef.ProviderOpId) {
	switch in.Op {
	case OpCall, OpClosure, OpFnItem, OpBind:
	default:
		return 0, 0
	}
	if int(in.Ent) >= len(p.R.Entities) {
		return 0, 0
	}
	if e := p.R.Entity(in.Ent); e.Kind != sem.EntFn || e.Flags&sem.EfStd == 0 {
		return 0, 0
	}
	prim, prov, _ := ResolveStdOp(p.R, in.Ent)
	return prim, prov
}
