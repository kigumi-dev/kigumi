package mir

import "kigumi/internal/sem"

// unownedRefs marks the references this frame holds no count of: a borrow of a
// value it has, and one a caller lent it. A field read takes its own count.
func (p *Program) unownedRefs(f *Func) []bool {
	if f.unownedCached {
		return f.unownedCache
	}
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
	f.unownedCache, f.unownedCached = out, true
	return out
}

// invalidateUnowned drops every function's cached unownedRefs; Verify defers
// this call so the cache never survives past the round that filled it.
func (p *Program) invalidateUnowned() {
	for _, f := range p.allFuncs() {
		f.unownedCache, f.unownedCached = nil, false
	}
}

func lendsOn(in Inst, out []bool) bool {
	switch in.Op {
	case OpBorrow:
		return true
	case OpAlias, OpMove:
		return len(in.Args) == 1 && out[in.Args[0]]
	case OpCopy:
		return !in.Share && len(in.Args) == 1 && out[in.Args[0]]
	}
	return false
}

// build.go's emit drops no reference, so a declared reference parameter keeps
// the caller's count; a type parameter unified to one is owned like any value.
func (p *Program) lentParam(in Inst, i int) bool {
	if in.Op == OpRecord {
		return p.borrowField(in.Ent, i)
	}
	t, ok := p.declaredParam(in, i)
	return ok && p.R.Types.Kind(t) == sem.KRef
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
func (p *Program) declaredParam(in Inst, i int) (sem.TypeID, bool) {
	prm, ok := p.declaredParamEnt(in, i)
	if !ok {
		return 0, false
	}
	return p.R.Entity(prm).Type, true
}

// declaredParamEnt is declaredParam's parameter itself, for a diagnostic naming it.
func (p *Program) declaredParamEnt(in Inst, i int) (sem.EntityID, bool) {
	switch in.Op {
	case OpCall:
	case OpCallValue:
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
