package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

type originEdge struct {
	dsts, srcs []LocalID
}

// heldOrigin is a parameter whose `&mut`s l's value may keep, so a store through them lands in the caller's places:
// l itself, or one the value flow carries into l by a copy, a read, a literal, a store, an element taken out or a call.
func (c *borrowCheck) heldOrigin(l LocalID) (LocalID, bool) {
	if slices.Contains(c.f.Params, l) {
		return l, true
	}
	o, ok := c.heldOrigins()[l]
	return o, ok
}

// heldRead is the parameter whose storage a reference read out of a value may point into, whatever else the holders
// say it points at: a value the parameter handed over, or what a reference local to one leads to.
func (c *borrowCheck) heldRead(l LocalID) (LocalID, bool) {
	r, _, _, ok := c.projectionPlace(l)
	if !ok || r == l || c.p.R.Types.Kind(c.f.Locals[l].Type) != sem.KRef {
		return 0, false
	}
	if p, ok := c.heldOrigin(r); ok {
		return p, true
	}
	if c.p.R.Types.Kind(c.f.Locals[r].Type) != sem.KRef {
		return 0, false
	}
	if r, ok = c.refRoot(r); !ok {
		return 0, false
	}
	return c.heldOrigin(r)
}

// heldOrigins settles from the instructions alone, so collect and the checks agree; a type naming an unsubstituted
// parameter (`Array.of`'s result) counts as keeping a `&mut`, so an unknown edge carries the origin on, never drops it.
func (c *borrowCheck) heldOrigins() map[LocalID]LocalID {
	if c.origins != nil {
		return c.origins
	}
	out := map[LocalID]LocalID{}
	c.origins = out
	keeps := func(l LocalID) bool {
		t := c.f.Locals[l].Type
		return c.keepsMutRef(t) || c.namesTypeParam(t)
	}
	for _, p := range c.f.Params {
		if c.keepsMutRef(c.f.Locals[p].Type) {
			out[p] = p
		}
	}
	if len(out) == 0 {
		return out
	}
	has := func(l LocalID) bool { _, ok := out[l]; return ok }
	var insts []Inst
	for _, blk := range c.f.Blocks {
		insts = append(insts, blk.Insts...)
	}
	defs := c.soleDefMap()
	edges := make([]*originEdge, len(insts))
	for changed := true; changed; {
		changed = false
		for i, in := range insts {
			if !slices.ContainsFunc(in.Args, has) {
				continue
			}
			if edges[i] == nil {
				e := c.originEdge(in, defs, keeps)
				edges[i] = &e
			}
			k := slices.IndexFunc(edges[i].srcs, has)
			if k < 0 {
				continue
			}
			for _, d := range edges[i].dsts {
				if !has(d) && keeps(d) {
					out[d], changed = out[edges[i].srcs[k]], true
				}
			}
		}
	}
	return out
}

// originEdge is where in may carry what its operands keep: its result, the place a store writes, and for a call each
// place an argument lends; a callee with a body passes on what its summaries say it may move out or store.
func (c *borrowCheck) originEdge(in Inst, defs map[LocalID]*Inst, keeps func(LocalID) bool) originEdge {
	var e originEdge
	switch {
	case in.Op == OpSetField || in.Op == OpSetIndex || in.Op == OpCellSet:
		return originEdge{dsts: writtenPlaces(c.f, in.Args[0], defs, keeps), srcs: in.Args[len(in.Args)-1:]}
	case in.Op == OpCall || in.Op == OpCallValue || in.Op == OpBuiltin || in.Op == OpFutureNew:
		h := c.bodyCallee(in)
		for k, a := range in.Args {
			if !keeps(a) {
				continue
			}
			if h == nil || c.paramMoves(h, k) {
				e.srcs = append(e.srcs, a)
			}
			if h == nil || c.paramStores(h, k) {
				e.dsts = append(e.dsts, writtenPlaces(c.f, a, defs, keeps)...)
			}
		}
	case !in.Op.Yields() || valueFlowFresh[in.Op]:
		return e
	default:
		e.srcs = in.Args
	}
	if in.Op.Yields() {
		e.dsts = append(e.dsts, in.Dst)
	}
	return e
}

// writtenPlaces are the named locals a write into or through a may reach: back through how a's value was made, every
// local it was read, copied or `&mut` borrowed from, as far as those keep a `&mut`; a shared borrow lends no write.
func writtenPlaces(f *Func, a LocalID, defs map[LocalID]*Inst, keeps func(LocalID) bool) []LocalID {
	var out []LocalID
	seen := map[LocalID]bool{}
	work := []LocalID{a}
	for len(work) > 0 {
		l := work[len(work)-1]
		work = work[:len(work)-1]
		if seen[l] || !keeps(l) {
			continue
		}
		seen[l] = true
		if f.Locals[l].Ent != 0 {
			out = append(out, l)
		}
		d := defs[l]
		if d == nil || valueFlowFresh[d.Op] || d.Op == OpBorrow && d.Index != 1 {
			continue
		}
		work = append(work, d.Args...)
	}
	return out
}
