package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// keptSlots are g's parameter positions: index, a local's declared parameter index or -1, and slot, that or the place
// after them a fn's witness or a closure's capture takes in a bind.
func (c *borrowCheck) keptSlots(g *midir.Func, declared []sem.EntityID) (index, slot func(midir.LocalID) int) {
	index = func(l midir.LocalID) int {
		if e := g.Locals[l].Ent; e != 0 && slices.Contains(g.Params, l) {
			return slices.Index(declared, e)
		}
		return -1
	}
	hidden := c.hiddenLocals(g.Ent)
	slot = func(l midir.LocalID) int {
		if i := index(l); i >= 0 || len(hidden) == 0 {
			return i
		}
		if w := slices.Index(hidden, g.Locals[l].Ent); w >= 0 && slices.Contains(g.Params, l) {
			return len(declared) + w
		}
		return -1
	}
	return index, slot
}

// hiddenLocals are the parameters of e its callers pass besides the declared ones: a fn's witnesses, a closure's captures.
func (c *borrowCheck) hiddenLocals(e sem.EntityID) []sem.EntityID {
	var out []sem.EntityID
	switch c.p.R.Entity(e).Kind {
	case sem.EntFn:
		for _, w := range c.p.R.Fn(e).Witnesses {
			out = append(out, w.Local)
		}
	case sem.EntClosure:
		for _, cp := range c.p.R.Closure(e).Captures {
			out = append(out, cp.Local)
		}
	}
	return out
}
