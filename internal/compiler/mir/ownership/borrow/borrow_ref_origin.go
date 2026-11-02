package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (state *Analyzer) refScopeOrigins(u sem.RefScopeUse) ([]refScopeKey, string, bool) {
	p := state.Input
	var out []refScopeKey
	if u.Target != 0 && len(u.Parts) == 0 && u.Results == 0 {
		for _, q := range u.Params {
			out = append(out, refScopeKey{u.Target, q})
		}
		return out, p.R.Entity(u.Target).Name, len(out) > 0
	}
	name, items := "function value", knownItems(u.Target)
	if u.Target != 0 {
		name = p.R.Entity(u.Target).Name
	}
	if u.Unprovable {
		return nil, name, false
	}
	if u.Target == 0 {
		items = state.useValueItems(u)
	}
	for range u.Results {
		items = state.returnedFnItems(items)
	}
	for _, e := range items.ents {
		ps := state.paramsOf(e)
		for _, slot := range u.Slots {
			// A callable without the slot has another type; only a join that over-approximates can bring it.
			if slot >= len(ps) {
				continue
			}
			if key := (refScopeKey{e, ps[slot]}); !slices.Contains(out, key) {
				out = append(out, key)
			}
		}
	}
	return out, name, items.known
}

// useValueItems are the fn items the value u erases can be, or hold at u.Parts.
func (state *Analyzer) useValueItems(u sem.RefScopeUse) fnItems {
	p := state.Input
	g := p.ByEnt[u.Owner]
	if g == nil {
		return fnItems{}
	}
	c := &borrowCheck{p: p, f: g, state: state}
	var locals []midir.LocalID
	if ent := p.R.Files[u.File].Uses[u.Node]; ent != 0 {
		for l, local := range g.Locals {
			if local.Ent == ent {
				locals = append(locals, midir.LocalID(l))
			}
		}
	}
	if len(locals) == 0 {
		for _, blk := range g.Blocks {
			for _, in := range blk.Insts {
				if in.Node == u.Node && in.Op.Yields() && (len(u.Parts) > 0 || c.slotFnType(g.Locals[in.Dst].Type) != 0) {
					locals = append(locals, in.Dst)
				}
			}
		}
	}
	if len(locals) == 0 {
		return fnItems{}
	}
	out := knownItems()
	for _, l := range locals {
		out = out.join(c.fnItemPath(l, u.Parts, 8))
	}
	return out
}

// returnedFnItems are the fn items a call of any of items can return, read off the return values of their bodies.
func (state *Analyzer) returnedFnItems(items fnItems) fnItems {
	if !items.known {
		return items
	}
	out := knownItems()
	for _, e := range items.ents {
		g := state.Input.ByEnt[e]
		if g == nil {
			return fnItems{}
		}
		c := &borrowCheck{p: state.Input, f: g, state: state}
		returns := false
		for _, blk := range g.Blocks {
			if blk.Term.Op != midir.TermReturn {
				continue
			}
			for _, a := range blk.Term.Args {
				returns, out = true, out.join(c.fnItemOrigins(a, 8))
			}
		}
		if !returns {
			return fnItems{}
		}
	}
	return out
}

func (state *Analyzer) paramsOf(e sem.EntityID) []sem.EntityID {
	if state.Input.R.Entity(e).Kind == sem.EntClosure {
		return state.Input.R.Closure(e).Params
	}
	return state.Input.R.Fn(e).Params
}
