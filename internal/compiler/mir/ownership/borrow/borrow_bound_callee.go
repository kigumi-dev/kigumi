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
				if in.Ent != g.Ent {
					continue
				}
				if in.Op != midir.OpCall || param >= len(in.Args) {
					return false
				}
				seen = true
				o := c.calleeOrigin(caller, defs, in.Args[param])
				if !c.boundArgQuiet(o, slot) {
					return false
				}
			}
		}
	}
	return seen
}

func (c *borrowCheck) boundArgQuiet(o calleeOrigin, slot int) bool {
	if o.body == nil {
		return false
	}
	i := len(o.captures) + slot
	s := c.keptOf(o.body.Ent)
	params := c.declaredParams(o.body.Ent)
	if i >= len(params) || i >= contentShift || s == nil || s.keepsResult(i) || s.movesOut(i) {
		return false
	}
	for dst := range s.stores {
		if s.storesInto(dst, i) || s.movesInto(dst, i) {
			return false
		}
	}
	return !slices.Contains(c.sharedSinks(o.body.Ent, nil), params[i]) && !c.boundOpaqueStores(o.body, i, map[boundSlot]bool{})
}
