package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
)

// callerParam: l is a parameter whose place the caller owns: a `&mut` referent, a `mut self` receiver, or a cell
// captured from the enclosing body.
func callerParam(p *ownquery.Input, f *midir.Func, l midir.LocalID) bool {
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
func callerPlace(p *ownquery.Input, f *midir.Func, defs map[midir.LocalID]*midir.Inst, l midir.LocalID) bool {
	for range 32 {
		if slices.Contains(f.Params, l) {
			return callerParam(p, f, l)
		}
		d := defs[l]
		if d == nil || len(d.Args) == 0 {
			return false
		}
		switch d.Op {
		case midir.OpField, midir.OpIndex, midir.OpPayload, midir.OpBorrow, midir.OpCopy, midir.OpAlias, midir.OpShare, midir.OpCellGet, midir.OpNewCell:
			l = d.Args[0]
		default:
			return false
		}
	}
	return false
}

// intoCaller lists the holders in puts into a caller's place of the checked body.
func (e *erasedReaders) intoCaller(in midir.Inst, caps map[midir.LocalID][]capturedPlace) []midir.LocalID {
	return e.callerStores(e.f, e.defs, in, func(l midir.LocalID) bool { _, ok := caps[l]; return ok })
}

// callerStores lists the held arguments in may put into a caller's place of f: a store through it, or a call handed
// both the value and such a place whose callee may store the one into the other, however deep the store happens.
func (e *erasedReaders) callerStores(f *midir.Func, defs map[midir.LocalID]*midir.Inst, in midir.Inst, held func(midir.LocalID) bool) []midir.LocalID {
	if v, ok := storeValue(in); ok {
		if held(v) && callerPlace(e.p, f, defs, in.Args[0]) {
			return []midir.LocalID{v}
		}
		return nil
	}
	if in.Op != midir.OpCall && in.Op != midir.OpCallValue || !slices.ContainsFunc(in.Args, func(a midir.LocalID) bool { return callerPlace(e.p, f, defs, a) }) {
		return nil
	}
	g := e.callee(in)
	var out []midir.LocalID
	for i, a := range in.Args {
		if held(a) && !(in.Op == midir.OpCallValue && i == 0) && (g == nil || e.fate(g, i).stores) {
			out = append(out, a)
		}
	}
	return out
}
