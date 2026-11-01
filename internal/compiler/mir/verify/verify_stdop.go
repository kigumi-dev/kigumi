package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

func (p *verifier) verifyStdOpTags(f *midir.Func) *Error {
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Prim != 0 || in.Prov != 0 {
				prim, prov := p.resolvedTags(in)
				if in.Prim != prim || in.Prov != prov {
					return fail("stdop", midir.BlockID(bid), i, "%s carries runtime op tags %d/%d, which its entity does not resolve to", in.Op, in.Prim, in.Prov)
				}
			}
			if in.DPrim != opdef.DPNone && (in.Op != midir.OpCall || in.DPrim != p.DirectPrimOf(f, in)) {
				return fail("stdop", midir.BlockID(bid), i, "%s carries direct result tag %d, which it does not take", in.Op, in.DPrim)
			}
		}
	}
	return nil
}

func (p *verifier) resolvedTags(in midir.Inst) (opdef.PrimitiveId, opdef.ProviderOpId) {
	switch in.Op {
	case midir.OpCall, midir.OpClosure, midir.OpFnItem, midir.OpBind:
	default:
		return 0, 0
	}
	if int(in.Ent) >= len(p.R.Entities) {
		return 0, 0
	}
	if e := p.R.Entity(in.Ent); e.Kind != sem.EntFn || e.Flags&sem.EfStd == 0 {
		return 0, 0
	}
	prim, prov, _ := opdef.ResolveStdOp(p.R, in.Ent)
	return prim, prov
}
