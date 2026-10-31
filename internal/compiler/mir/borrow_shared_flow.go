package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// flowMemo holds ruledOut per local; a local met again on a cycle rules out nothing.
type flowMemo map[LocalID][]LocalID

// flowOwners are the owners of a's holder its value flow does not rule out (ruledOut).
func (c *borrowCheck) flowOwners(a LocalID, memo flowMemo) []ownerRef {
	b, ok := c.holders[a]
	if !ok {
		return nil
	}
	out := c.ruledOut(a, memo)
	return slices.DeleteFunc(slices.Clone(b.owners), func(o ownerRef) bool { return slices.Contains(out, o.owner) })
}

// ruledOut are the owners of a's holder its value flow proves never reach it: only a call whose result cannot carry them
// proves it (callRuledOut), copies and builders pass that on, and a local the walk cannot follow rules out nothing.
func (c *borrowCheck) ruledOut(a LocalID, memo flowMemo) []LocalID {
	if out, done := memo[a]; done {
		return out
	}
	memo[a] = nil
	defs := c.defsOf(a)
	if _, ok := c.holders[a]; !ok || len(defs) == 0 || c.f.Locals[a].Ent != 0 && (c.targets[a] || c.writtenThrough(a)) {
		return nil
	}
	var out []LocalID
	for i, d := range defs {
		r := c.defRuledOut(*d, memo)
		if i > 0 {
			r = slices.DeleteFunc(r, func(o LocalID) bool { return !slices.Contains(out, o) })
		}
		out = r
	}
	memo[a] = out
	return out
}

// defRuledOut is what definition d rules out: an instruction whose result holds only what its operands hold (bodyRoles'
// roleCarry: copies, builders, projections, cells, boxes) passes on what they rule out, as a future does its captures.
func (c *borrowCheck) defRuledOut(d Inst, memo flowMemo) []LocalID {
	switch {
	case d.Op == OpCall || d.Op == OpCallValue:
		return c.callRuledOut(d, memo)
	case d.Op == OpFutureNew:
		return c.partsRuledOut(d.Args[1:], memo)
	case bodyRoles[d.Op] == roleCarry:
		return c.partsRuledOut(d.Args, memo)
	}
	return nil
}

// partsRuledOut rules out an owner some part lists only when every part listing it rules it out.
func (c *borrowCheck) partsRuledOut(parts []LocalID, memo flowMemo) []LocalID {
	var listed, kept []LocalID
	for _, x := range parts {
		r := c.ruledOut(x, memo)
		for _, o := range c.holderOwners(x) {
			listed = append(listed, o.owner)
			if !slices.Contains(r, o.owner) {
				kept = append(kept, o.owner)
			}
		}
	}
	return slices.DeleteFunc(listed, func(o LocalID) bool { return slices.Contains(kept, o) })
}

// callRuledOut are the owners of call in's result that no operand the result may carry (resultCarries) explains; none
// when the callee or a lent operand's places are unknown.
func (c *borrowCheck) callRuledOut(in Inst, memo flowMemo) []LocalID {
	carry, ok := c.resultCarries(in)
	if !ok {
		return nil
	}
	var explained []LocalID
	for k, a := range in.Args {
		m := carry(k)
		if m&carryValue != 0 {
			for _, o := range c.flowOwners(a, memo) {
				explained = append(explained, o.owner)
			}
		}
		if m&(carryContents|carryPlace) == 0 {
			continue
		}
		places, ok := c.lentPlaces(a)
		if !ok {
			return nil
		}
		for _, p := range places {
			if m&carryContents != 0 {
				for _, o := range c.flowOwners(p, memo) {
					explained = append(explained, o.owner)
				}
			}
			if m&carryPlace != 0 {
				explained = append(explained, p)
			}
		}
	}
	var out []LocalID
	for _, o := range c.holderOwners(in.Dst) {
		if !slices.Contains(explained, o.owner) {
			out = append(out, o.owner)
		}
	}
	return out
}

// lentPlaces are the places operand a lends a call: back through a write-back cell, a retain, copies and the borrow or
// view that made it, a by-value operand being its own place; false when the chain leads anywhere else.
func (c *borrowCheck) lentPlaces(a LocalID) ([]LocalID, bool) {
	for range 16 {
		if c.p.R.Types.Kind(c.f.Locals[a].Type) != sem.KRef && !c.f.Locals[a].Cell || slices.Contains(c.f.Params, a) {
			out := []LocalID{a}
			if root, _, _, ok := c.projectionPlace(a); ok && root != a {
				out = append(out, root)
			}
			return out, true
		}
		d := c.definition(a)
		if d == nil || len(d.Args) == 0 {
			return nil, false
		}
		switch d.Op {
		case OpNewCell, OpShare, OpCopy, OpMove, OpAlias, OpLendBox:
			a = d.Args[0]
		case OpBorrow:
			root, _, _, ok := c.patternPlace(d.Args[0])
			return []LocalID{root}, ok
		default:
			return nil, false
		}
	}
	return nil, false
}
