package borrow

import (
	"slices"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) boundCalleeQuiet(g *midir.Func, param, slot int) bool {
	e := c.p.R.Entity(g.Ent)
	if e.Kind != sem.EntFn || e.Vis.Level != sem.VisPrivate || c.p.R.Fn(g.Ent).Export != "" {
		return false
	}
	if param >= len(g.Params) || g.Locals[g.Params[param]].Cell || len(allDefs(g)[g.Params[param]]) > 0 {
		return false
	}
	seen := false
	for _, caller := range c.p.Funcs() {
		defs := mirflow.SoleDefs(caller)
		for _, blk := range caller.Blocks {
			for _, in := range blk.Insts {
				if !c.namesBody(in.Ent, g.Ent) {
					continue
				}
				if in.Op != midir.OpCall || param >= len(in.Args) {
					return false
				}
				seen = true
				if !c.siteArgQuiet(caller, defs, in.Args[param], slot) {
					return false
				}
			}
		}
	}
	return seen
}

// namesBody reports whether an instruction naming e may run body ent: e itself, or an interface requirement whose
// implementers include ent or are not settled.
func (c *borrowCheck) namesBody(e, ent sem.EntityID) bool {
	if e == ent {
		return true
	}
	if e == 0 || !c.ifaceRequirement(e) {
		return false
	}
	ws, settled := c.p.R.SealedWitnesses(e)
	return !settled || slices.Contains(ws, ent)
}

// siteArgQuiet reports whether every callee the callable argument a of caller may be keeps nothing it is given at slot.
func (c *borrowCheck) siteArgQuiet(caller *midir.Func, defs map[midir.LocalID]*midir.Inst, a midir.LocalID, slot int) bool {
	origins, ok := c.calleeOrigins(caller, defs, a)
	return ok && !slices.ContainsFunc(origins, func(o calleeOrigin) bool { return !c.boundArgQuiet(o, slot) })
}

func (c *borrowCheck) boundArgQuiet(o calleeOrigin, slot int) bool {
	if o.body == nil {
		return false
	}
	i, ok := c.argDeclared(o, slot)
	s := c.keptOf(o.body.Ent)
	params := c.declaredParams(o.body.Ent)
	if !ok || i >= len(params) || i >= contentShift || s == nil || s.keepsResult(i) || s.movesOut(i) {
		return false
	}
	// The kept summary follows declared parameters only, so a store into a closure's capture shows in the body's sites.
	if c.p.R.Entity(o.ent).Kind == sem.EntClosure && o.nbound() > 0 && c.paramOut(o.body, o.argParam(slot)) {
		return false
	}
	for dst := range s.stores {
		if s.storesInto(dst, i) || s.movesInto(dst, i) {
			return false
		}
	}
	return !slices.Contains(c.sharedSinks(o.body.Ent, nil), params[i]) && !c.boundOpaqueStores(o.body, i, map[boundSlot]bool{})
}
