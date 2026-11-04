package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type witnessKey struct {
	fn sem.EntityID
	w  int
}

// witnessUse is one place a fn is named: a call or a bind whose witness arguments start at first; escape for a use
// that passes it on unbound (a fn item, a method value).
type witnessUse struct {
	g      *midir.Func
	args   []midir.LocalID
	first  int
	escape bool
}

func (c *borrowCheck) witnessUses() map[sem.EntityID][]witnessUse {
	m := &c.state.refKeeps
	if m.uses != nil {
		return m.uses
	}
	m.uses = map[sem.EntityID][]witnessUse{}
	for _, g := range c.p.Funcs() {
		for _, blk := range g.Blocks {
			for _, in := range blk.Insts {
				if in.Ent == 0 || c.p.R.Entity(in.Ent).Kind != sem.EntFn || len(c.hiddenLocals(in.Ent)) == 0 {
					continue
				}
				u := witnessUse{g: g, args: in.Args}
				switch {
				case in.Op == midir.OpCall:
					u.first = len(c.declaredParams(in.Ent))
				case in.Op == midir.OpBind:
				default:
					u.escape = true
				}
				m.uses[in.Ent] = append(m.uses[in.Ent], u)
			}
		}
	}
	return m.uses
}

// witnessSet are the callables witness w of fn e is bound to across the program's uses of e, through callers that pass
// their own witness on; nil when some use cannot be followed. A fn the program never instantiates has none.
func (c *borrowCheck) witnessSet(e sem.EntityID, w int) []boundFn {
	m := &c.state.refKeeps
	key := witnessKey{e, w}
	if fns, ok := m.witnesses[key]; ok {
		return fns
	}
	fns, ok := c.witnessReach(key, map[witnessKey]bool{})
	if !ok {
		fns = nil
	} else if fns == nil {
		fns = []boundFn{}
	}
	if m.witnesses == nil {
		m.witnesses = map[witnessKey][]boundFn{}
	}
	m.witnesses[key] = fns
	if witnessProbe != nil {
		var wide []boundFn
		for _, impl := range c.implementers(c.p.R.Fn(e).Witnesses[w].Req) {
			wide = append(wide, boundFn{ent: impl})
		}
		witnessProbe(c, fns, wide)
	}
	return fns
}

// witnessProbe, when a test sets it, sees each witnessSet answer next to every implementation of its requirement.
var witnessProbe func(c *borrowCheck, narrow, wide []boundFn)

func (c *borrowCheck) witnessReach(key witnessKey, seen map[witnessKey]bool) ([]boundFn, bool) {
	if seen[key] {
		return nil, true
	}
	seen[key] = true
	var out []boundFn
	for _, u := range c.witnessUses()[key.fn] {
		if u.escape || u.first+key.w >= len(u.args) {
			return nil, false
		}
		failed := false
		defs := allDefs(u.g)
		n := len(c.declaredParams(u.g.Ent))
		v := defView{defs: func(l midir.LocalID) []*midir.Inst { return defs[l] }, param: func(l midir.LocalID) []boundFn {
			hidden := c.hiddenLocals(u.g.Ent)
			w := slices.Index(hidden, u.g.Locals[l].Ent)
			if c.p.R.Entity(u.g.Ent).Kind != sem.EntFn || w < 0 || !slices.Contains(u.g.Params, l) || slices.Index(u.g.Params, l) < n {
				return nil
			}
			fns, ok := c.witnessReach(witnessKey{u.g.Ent, w}, seen)
			failed = failed || !ok
			return append([]boundFn{}, fns...)
		}}
		fns := c.refItems(v, u.args[u.first+key.w], 8)
		if fns == nil || failed {
			return nil, false
		}
		out = append(out, fns...)
	}
	return out, true
}
