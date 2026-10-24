package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// callerStorage reports whether l is the caller's storage: a caller place, a reference into or a part of one, a binding
// that shares one's heap object, or a part of a call result drawn from one; a binding of an inline copy is this frame's.
func (c *borrowCheck) callerStorage(l LocalID) bool {
	return c.callerStorageFrom(l, false, 0, nil)
}

// callerStorageFrom sets *root to the caller place the walk ends at, when it ends at one and not at a call.
func (c *borrowCheck) callerStorageFrom(l LocalID, projected bool, depth int, root *[]LocalID) bool {
	for range len(c.f.Locals) {
		if c.callerPlace(l) {
			if root != nil {
				*root = []LocalID{l}
			}
			return true
		}
		d := soleDef(c.f, l)
		if d == nil || len(d.Args) == 0 {
			return false
		}
		switch d.Op {
		case OpCopy, OpMove, OpAlias:
			if c.f.Locals[l].Ent != 0 && c.inlineValue(c.f.Locals[l].Type) {
				return false
			}
		case OpBorrow, OpUnbox:
		case OpField, OpIndex, OpPayload:
			projected = true
		case OpBuiltin:
			if !iterElement(d) {
				return false
			}
			projected = true
		case OpCall, OpCallValue:
			return depth < 8 && c.callFromCaller(d, projected, depth+1)
		default:
			return false
		}
		l = d.Args[0]
	}
	return false
}

func iterElement(d *Inst) bool {
	return d.Op == OpBuiltin && (d.Str == "iter.at" || d.Str == "iter.at_ref")
}

// inlineValue reports whether a binding of a t holds its own copy in a frame slot, a scalar or a count-free direct aggregate whose
// borrow points at that slot; any other value, a counted Option or Result too, shares a heap object with its source.
func (c *borrowCheck) inlineValue(t sem.TypeID) bool {
	u := c.p.R.UnderlyingRepr(t)
	return u == sem.TyUnit || c.p.R.RegScalar(u) || sem.WideScalar(u) || c.p.R.CountFree(u)
}

// callFromCaller reports whether what a call returns, or a part read out of it, is the caller's storage: the result
// borrows only the caller's storage, or the part has a type parameter that every argument it can come from brings in.
func (c *borrowCheck) callFromCaller(d *Inst, projected bool, depth int) bool {
	if !projected && c.p.R.Types.Kind(c.f.Locals[d.Dst].Type) != sem.KRef {
		return false
	}
	if h, ok := c.holders[d.Dst]; ok && !h.empty {
		return !h.hasOwner || !slices.ContainsFunc(h.owners, func(o ownerRef) bool { return !c.callerStorageFrom(o.owner, false, depth, nil) })
	}
	if !projected || d.Ent == 0 || c.p.R.Entity(d.Ent).Kind != sem.EntFn {
		return false
	}
	info := c.p.R.Fn(d.Ent)
	want := c.typeParamsOf(c.p.R.Types.Node(info.Sig).Elem)
	recvIdx := 0
	if d.Op == OpCallValue {
		recvIdx = 1
	}
	slots := c.callSlots(info, d.Args, recvIdx)
	c.linkSlots(info, slots)
	found := false
	for _, s := range slots {
		if !sharesParam(s.params, want) {
			continue
		}
		if !c.callerStorageFrom(s.arg, false, depth, nil) {
			return false
		}
		found = true
	}
	return found
}
