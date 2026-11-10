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
	en := c.witnessSolve(witnessKey{e, w})
	switch {
	case !en.ok:
		return nil
	case en.fns == nil:
		return []boundFn{}
	}
	return en.fns
}

// witnessProbe, when a test sets it, sees each witnessSet answer next to every implementation of its requirement.
var witnessProbe func(c *borrowCheck, narrow, wide []boundFn)

// witnessReach joins what each use of key's fn binds witness key.w to, reading a caller's own witness through
// witnessSolve; false when some use cannot be followed.
func (c *borrowCheck) witnessReach(key witnessKey) ([]boundFn, bool) {
	var out []boundFn
	for _, u := range c.witnessUses()[key.fn] {
		if u.escape || u.first+key.w >= len(u.args) {
			return nil, false
		}
		failed := false
		defs := allDefs(u.g)
		n := len(c.declaredParams(u.g.Ent))
		v := defView{defs: func(l midir.LocalID) []*midir.Inst { return defs[l] }, params: u.g.Params, param: func(l midir.LocalID) []boundFn {
			hidden := c.hiddenLocals(u.g.Ent)
			w := slices.Index(hidden, u.g.Locals[l].Ent)
			if c.p.R.Entity(u.g.Ent).Kind != sem.EntFn || w < 0 || !slices.Contains(u.g.Params, l) || slices.Index(u.g.Params, l) < n {
				return nil
			}
			en := c.witnessSolve(witnessKey{u.g.Ent, w})
			failed = failed || !en.ok
			return append([]boundFn{}, en.fns...)
		}}
		fns := c.refItems(v, u.args[u.first+key.w], 8)
		if fns == nil || failed {
			return nil, false
		}
		out = append(out, fns...)
	}
	return uniqueFns(out), true
}
