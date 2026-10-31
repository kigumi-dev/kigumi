package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

func (state *borrowAnalysis) refScopeOrigins(u sem.RefScopeUse) ([]refScopeKey, string, bool) {
	p := state.Program
	var out []refScopeKey
	if u.Target != 0 {
		for _, q := range u.Params {
			out = append(out, refScopeKey{u.Target, q})
		}
		return out, p.R.Entity(u.Target).Name, len(out) > 0
	}
	const name = "function value"
	g := p.ByEnt[u.Owner]
	if g == nil {
		return nil, name, false
	}
	c := &borrowCheck{p: p, f: g, state: state}
	var locals []LocalID
	if ent := p.R.Files[u.File].Uses[u.Node]; ent != 0 {
		for l, local := range g.Locals {
			if local.Ent == ent {
				locals = append(locals, LocalID(l))
			}
		}
	}
	if len(locals) == 0 {
		for _, blk := range g.Blocks {
			for _, in := range blk.Insts {
				if in.Node == u.Node && in.Op.Yields() && c.slotFnType(g.Locals[in.Dst].Type) != 0 {
					locals = append(locals, in.Dst)
				}
			}
		}
	}
	for _, l := range locals {
		ents, ok := c.fnItemOrigins(l, 8)
		if !ok || len(ents) == 0 {
			return nil, name, false
		}
		for _, e := range ents {
			ps := p.R.Fn(e).Params
			for _, slot := range u.Slots {
				if slot >= len(ps) {
					return nil, name, false
				}
				if key := (refScopeKey{e, ps[slot]}); !slices.Contains(out, key) {
					out = append(out, key)
				}
			}
		}
	}
	return out, name, len(out) > 0
}
