package mir

import (
	"fmt"
	"slices"
	"strings"

	"kigumi/internal/sem"
)

// keptCall is what call in hands its result and, per operand i, that operand's place: by keptCallees' summaries, for a
// body-less primitive only what moved hands on from a lent operand unless the result can hold a reference, else all.
func (c *borrowCheck) keptCall(k *keptCtx, in Inst) (uint64, func(int) uint64) {
	var from uint64
	for _, a := range in.Args {
		from |= k.facts[a]
	}
	every := func(int) uint64 { return from }
	ents, off := c.keptCallees(k, in)
	args := in.Args[min(off, len(in.Args)):]
	plainResult := c.ifaceRequirement(in.Ent) && !c.holdsBorrow(k.g.Locals[in.Dst].Type) && !c.holdsDeferred(k.g.Locals[in.Dst].Type)
	type callee struct {
		s *keptSummary
		h *Func
		n int
	}
	var cs []callee
	var signatures []func(int) uint64
	var res uint64
	for _, e := range ents {
		if c.p.ByEnt[e] == nil {
			r, lent := c.keptSignature(k, in, e, args, off)
			res |= r
			signatures = append(signatures, lent)
			continue
		}
		s := c.keptBound(e, c.argBind(k, e, args))
		if s == nil {
			return from, every
		}
		cs = append(cs, callee{s, c.p.ByEnt[e], len(c.declaredParams(e))})
	}
	if cs == nil && signatures == nil {
		return from, every
	}
	for j, a := range args {
		for _, ce := range cs {
			switch {
			case plainResult && (c.p.R.Types.Kind(k.g.Locals[a].Type) == sem.KRef || k.g.Locals[a].Cell):
				if j >= ce.n || ce.s.keepsResult(j) || ce.s.movesOut(j) || j < len(ce.h.Params) && c.paramMoves(ce.h, j) {
					res |= c.moved(k, a)
				}
			case j >= ce.n || ce.s.keepsResult(j):
				res |= k.facts[a]
			case c.lendsPlace(k.g, a) && (ce.s.movesOut(j) || j < len(ce.h.Params) && c.paramMoves(ce.h, j)):
				res |= c.moved(k, a)
			}
		}
	}
	return res, func(i int) uint64 {
		t := i - off
		if t < 0 {
			return k.facts[in.Args[i]]
		}
		out := k.facts[args[t]]
		for _, lent := range signatures {
			out |= lent(i)
		}
		for j, a := range args {
			for _, ce := range cs {
				switch {
				case t >= ce.n || j >= ce.n || ce.s.storesInto(t, j):
					out |= k.facts[a]
				case c.lendsPlace(k.g, a) && ce.s.movesInto(t, j):
					out |= c.moved(k, a)
				}
			}
		}
		return out
	}
}

// keptCallees are the sync fn items call in runs and the index of its first argument; nil for an async or unknown callee.
func (c *borrowCheck) keptCallees(k *keptCtx, in Inst) ([]sem.EntityID, int) {
	var ents []sem.EntityID
	off := 0
	switch {
	case in.Op == OpCall && in.Ent != 0:
		ents = []sem.EntityID{in.Ent}
	case in.Op == OpCallValue && in.Ent != 0:
		ents, off = []sem.EntityID{in.Ent}, 1
	case in.Op == OpCallValue && in.Ent == 0 && len(in.Args) > 0:
		ents, off = c.itemsIn(k, in.Args[0], 8), 1
	}
	ents = c.resolveKeptCallees(ents)
	if slices.ContainsFunc(ents, func(e sem.EntityID) bool { return c.p.R.Entity(e).Kind != sem.EntFn || c.p.isAsyncBody(e) }) {
		return nil, 0
	}
	return ents, off
}

// itemsIn are the fn items local l of k's body holds: through copies, or bound to the parameter it is; nil if any
// value it can hold is something else.
func (c *borrowCheck) itemsIn(k *keptCtx, l LocalID, depth int) []sem.EntityID {
	defs := k.defs[l]
	if i := k.index(l); len(defs) == 0 && i >= 0 {
		return k.bind[i]
	}
	if depth == 0 || len(defs) == 0 || k.g.Locals[l].Cell {
		return nil
	}
	var out []sem.EntityID
	for _, d := range defs {
		var ents []sem.EntityID
		switch {
		case d.Op == OpFnItem:
			ents = []sem.EntityID{d.Ent}
		case (d.Op == OpCopy || d.Op == OpMove || d.Op == OpAlias) && len(d.Args) > 0:
			ents = c.itemsIn(k, d.Args[0], depth-1)
		}
		if ents == nil {
			return nil
		}
		out = append(out, ents...)
	}
	return out
}

// argBind binds each parameter of e whose argument in k's body is known fn items to them.
func (c *borrowCheck) argBind(k *keptCtx, e sem.EntityID, args []LocalID) map[int][]sem.EntityID {
	var out map[int][]sem.EntityID
	for j, a := range args {
		if j >= len(c.declaredParams(e)) || c.fnResult(k.g.Locals[a].Type) == 0 {
			continue
		}
		if ents := c.itemsIn(k, a, 8); ents != nil {
			if out == nil {
				out = map[int][]sem.EntityID{}
			}
			out[j] = ents
		}
	}
	return out
}

// siteBind is argBind for a call in the body being checked, args being the callee's arguments.
func (c *borrowCheck) siteBind(e sem.EntityID, args []LocalID) map[int][]sem.EntityID {
	var out map[int][]sem.EntityID
	for j, a := range args {
		if j >= len(c.declaredParams(e)) || c.fnResult(c.f.Locals[a].Type) == 0 {
			continue
		}
		if ents, ok := c.fnItemOrigins(a, 8); ok {
			if out == nil {
				out = map[int][]sem.EntityID{}
			}
			out[j] = ents
		}
	}
	return out
}

func bindKey(bind map[int][]sem.EntityID) string {
	var parts []string
	for j, ents := range bind {
		parts = append(parts, fmt.Sprint(j, ents))
	}
	slices.Sort(parts)
	return strings.Join(parts, ";")
}
