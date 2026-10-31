package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// lentRefs computes which references hold no count in the current body.
func (p *Program) lentRefs(f *midir.Func) []bool {
	out := make([]bool, len(f.Locals))
	for _, prm := range f.Params {
		out[prm] = p.R.Types.Kind(f.Locals[prm].Type) == sem.KRef
	}
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !in.Op.Yields() || out[in.Dst] || !lendsOn(in, out) {
					continue
				}
				out[in.Dst], changed = true, true
			}
		}
	}
	return out
}

func lendsOn(in midir.Inst, out []bool) bool {
	switch in.Op {
	case midir.OpBorrow, midir.OpLendBox:
		return true
	case midir.OpAlias, midir.OpMove:
		return len(in.Args) == 1 && out[in.Args[0]]
	case midir.OpCopy:
		return !in.Share && len(in.Args) == 1 && out[in.Args[0]]
	case midir.OpUnboxDirect:
		return in.Lent && out[in.Args[0]]
	}
	return false
}

// build.go's emit drops no reference, so a declared reference parameter keeps
// the caller's count; a type parameter unified to one is owned like any value.
func (p *Program) lentParam(in midir.Inst, i int) bool {
	if in.Op == midir.OpRecord {
		return p.borrowField(in.Ent, i)
	}
	if in.Op == midir.OpCall {
		if g := p.ByEnt[in.Ent]; g != nil && i < len(g.Params) && g.Locals[g.Params[i]].Borrowed {
			return true
		}
	}
	t, ok := p.declaredParam(in, i)
	return ok && p.R.Types.Kind(t) == sem.KRef
}

// A holder keeps no count of a reference stored in it, as borrowField says of records.
func (p *Program) lentStore(f *midir.Func, in midir.Inst, i int) bool {
	return in.Op == midir.OpSetField && i == 1 && p.R.Types.Kind(f.Locals[in.Args[1]].Type) == sem.KRef
}

// A record does not own what a reference-typed field holds, so building one leaves the count with the caller.
func (p *Program) borrowField(rec sem.EntityID, i int) bool {
	if rec == 0 || p.R.Entity(rec).Kind != sem.EntType {
		return false
	}
	fields := p.R.TypeDecl(rec).Fields
	return i < len(fields) && p.R.Types.Kind(p.R.Entity(fields[i]).Type) == sem.KRef
}

// declaredParam is the type before instantiation, not the argument's actual type.
// A receiver yields false; its convention comes from RecvKind, not a type.
func (p *Program) declaredParam(in midir.Inst, i int) (sem.TypeID, bool) {
	prm, ok := p.declaredParamEnt(in, i)
	if !ok {
		return 0, false
	}
	return p.R.Entity(prm).Type, true
}

// declaredParamEnt is declaredParam's parameter itself, for a diagnostic naming it.
func (p *Program) declaredParamEnt(in midir.Inst, i int) (sem.EntityID, bool) {
	switch in.Op {
	case midir.OpCall:
	case midir.OpCallValue:
		i--
	default:
		return 0, false
	}
	if in.Ent == 0 || p.R.Entity(in.Ent).Kind != sem.EntFn {
		return 0, false
	}
	info := p.R.Fn(in.Ent)
	if info.Recv != sem.RecvNone {
		i--
	}
	if i < 0 || i >= len(info.Params) {
		return 0, false
	}
	return info.Params[i], true
}
