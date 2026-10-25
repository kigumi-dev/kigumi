package mir

import "slices"

// callerParam: l is a parameter whose place the caller owns: a `&mut` referent, a `mut self` receiver, or a cell
// captured from the enclosing body.
func callerParam(p *Program, f *Func, l LocalID) bool {
	if !slices.Contains(f.Params, l) {
		return false
	}
	loc := f.Locals[l]
	if _, mut := p.R.MutRefElem(loc.Type); mut {
		return true
	}
	return loc.Cell && loc.Ent != 0 && (p.R.IsMutSelf(loc.Ent) || p.R.Entity(loc.Ent).Parent != f.Ent)
}

// callerPlace: l is, or is a part of, a place a caller parameter reaches.
func callerPlace(p *Program, f *Func, defs map[LocalID]*Inst, l LocalID) bool {
	for range 32 {
		if slices.Contains(f.Params, l) {
			return callerParam(p, f, l)
		}
		d := defs[l]
		if d == nil || len(d.Args) == 0 {
			return false
		}
		switch d.Op {
		case OpField, OpIndex, OpPayload, OpBorrow, OpCopy, OpAlias, OpShare, OpCellGet, OpNewCell:
			l = d.Args[0]
		default:
			return false
		}
	}
	return false
}

// intoCaller lists the holders in puts into a caller's place of the checked body.
func (e *erasedReaders) intoCaller(in Inst, caps map[LocalID][]capturedPlace) []LocalID {
	return e.callerStores(e.f, e.defs, in, func(l LocalID) bool { _, ok := caps[l]; return ok })
}

// callerStores lists the held arguments in may put into a caller's place of f: a store through it, or a call handed
// both the value and such a place whose callee may store the one into the other, however deep the store happens.
func (e *erasedReaders) callerStores(f *Func, defs map[LocalID]*Inst, in Inst, held func(LocalID) bool) []LocalID {
	if v, ok := storeValue(in); ok {
		if held(v) && callerPlace(e.p, f, defs, in.Args[0]) {
			return []LocalID{v}
		}
		return nil
	}
	if in.Op != OpCall && in.Op != OpCallValue || !slices.ContainsFunc(in.Args, func(a LocalID) bool { return callerPlace(e.p, f, defs, a) }) {
		return nil
	}
	g := e.callee(in)
	var out []LocalID
	for i, a := range in.Args {
		if held(a) && !(in.Op == OpCallValue && i == 0) && (g == nil || e.fate(g, i).stores) {
			out = append(out, a)
		}
	}
	return out
}
