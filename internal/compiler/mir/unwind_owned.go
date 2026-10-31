package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// A local that only holds a constant, the unit value or a never value owns nothing to release.
func immortalLocals(f *midir.Func, r *sem.Result) map[midir.LocalID]bool {
	imm := map[midir.LocalID]bool{}
	other := map[midir.LocalID]bool{}
	for _, prm := range f.Params {
		other[prm] = true
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() {
				continue
			}
			if in.Op == midir.OpConst || in.Op == midir.OpUnit {
				imm[in.Dst] = true
			} else {
				other[in.Dst] = true
			}
		}
	}
	for l := range f.Locals {
		if t := f.Locals[l].Type; t == sem.TyUnit || t == sem.TyNever {
			imm[midir.LocalID(l)] = true
			delete(other, midir.LocalID(l))
		}
	}
	for l := range other {
		delete(imm, l)
	}
	return imm
}

// An async body's named locals are swept from its frame, so the plan covers only temporaries held there.
func unwindOwned(f *midir.Func, r *sem.Result, candidates bool) map[midir.LocalID]bool {
	owned := owningLocals(f, r)
	for l := range immortalLocals(f, r) {
		owned[l] = false
	}
	if !f.Async {
		return owned
	}
	related := map[midir.LocalID]bool{}
	for dst, src := range aliasParents(f) {
		related[dst], related[src] = true, true
	}
	for _, prm := range f.Params {
		related[prm] = true
	}
	for l := range owned {
		keep := owned[l] && f.Locals[l].Ent == 0 && !related[l] && !r.RegScalar(f.Locals[l].Type)
		if keep && !candidates {
			keep = int(l) < len(f.Slot) && f.Slot[l] >= 0
		}
		owned[l] = keep
	}
	return owned
}

// A parameter the body only borrows stays with the caller, whose own frame releases it.
func (p *Program) lentParamLocals(f *midir.Func) map[midir.LocalID]bool {
	if f.Async {
		return nil
	}
	lent := map[midir.LocalID]bool{}
	isFn := f.Ent != 0 && p.R.Entity(f.Ent).Kind == sem.EntFn
	for i, prm := range f.Params {
		if isFn && p.ParamLent(f.Ent, i) || p.CaptureLent(f, i) {
			lent[prm] = true
		}
	}
	return lent
}
