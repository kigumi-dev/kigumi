package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// build.go's emit drops no reference, so a declared reference parameter keeps
// the caller's count; a type parameter unified to one is owned like any value.
func (p *Program) LentParam(in midir.Inst, i int) bool {
	if in.Op == midir.OpRecord {
		return p.BorrowField(in.Ent, i)
	}
	if in.Op == midir.OpCall {
		if g := p.ByEnt[in.Ent]; g != nil && i < len(g.Params) && g.Locals[g.Params[i]].Borrowed {
			return true
		}
	}
	t, ok := p.DeclaredParam(in, i)
	return ok && p.R.Types.Kind(t) == sem.KRef
}

// A holder keeps no count of a reference stored in it, as BorrowField says of records.
func (p *Program) LentStore(f *midir.Func, in midir.Inst, i int) bool {
	return in.Op == midir.OpSetField && i == 1 && p.R.Types.Kind(f.Locals[in.Args[1]].Type) == sem.KRef
}

// A record does not own what a reference-typed field holds, so building one leaves the count with the caller.
func (p *Program) BorrowField(rec sem.EntityID, i int) bool {
	if rec == 0 || p.R.Entity(rec).Kind != sem.EntType {
		return false
	}
	fields := p.R.TypeDecl(rec).Fields
	return i < len(fields) && p.R.Types.Kind(p.R.Entity(fields[i]).Type) == sem.KRef
}

// DeclaredParam is the type before instantiation, not the argument's actual type.
// A receiver yields false; its convention comes from RecvKind, not a type.
func (p *Program) DeclaredParam(in midir.Inst, i int) (sem.TypeID, bool) {
	prm, ok := ownquery.DeclaredParamEnt(p.R, in, i)
	if !ok {
		return 0, false
	}
	return p.R.Entity(prm).Type, true
}
