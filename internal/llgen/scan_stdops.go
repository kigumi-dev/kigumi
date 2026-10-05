package llgen

import (
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// rewrittenVariants lists the variants no site may share: rt_core.c's
// OP_OPTION_TAKE turns a Some into a None through the receiver's own storage.
func rewrittenVariants(p *mir.Program) map[sem.EntityID]bool {
	out := map[sem.EntityID]bool{}
	if !reachesStdOp(p, "OPTION_TAKE") {
		return out
	}
	opt := p.R.Types.OptionEnt()
	if opt == 0 {
		return out
	}
	for _, v := range p.R.TypeDecl(opt).Variants {
		if len(p.R.Variant(v).Payload) > 0 {
			out[v] = true
		}
	}
	return out
}

// reachesStdOp reports whether any instruction names a std function the
// registry resolves to op, whether it is called or taken as a value.
func reachesStdOp(p *mir.Program, op string) bool {
	want, ok := mir.StdOpIdOf(op)
	if !ok {
		return false
	}
	for _, f := range p.Funcs {
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if id, ok := in.StdOp(); ok && id == want {
					return true
				}
				if in.Ent == 0 || !p.IsStdPrimitive(in.Ent) {
					continue
				}
				if id, ok := mir.StdOpFor(mir.StdKey(p.R, in.Ent, true)); ok && id == want {
					return true
				}
			}
		}
	}
	return false
}
