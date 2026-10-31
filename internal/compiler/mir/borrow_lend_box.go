package mir

import "kigumi/internal/compiler/sem"

// lendBoxInfo makes an `&Interface` view a borrow of its source local as well: the view
// lives in this frame, so it may not be returned or stored past it.
func (c *borrowCheck) lendBoxInfo(in Inst) *borrowInfo {
	out := &borrowInfo{hasOwner: true, node: in.Node}
	if b, ok := c.holders[in.Args[0]]; ok && b.hasOwner {
		out.owners = append(out.owners, b.owners...)
	}
	out.owners = append(out.owners, ownerRef{owner: in.Args[0], exact: true}, ownerRef{owner: in.Dst, exact: true, view: true})
	return out
}

// checkViewReuse rejects a `lendbox` run again while a view it made on an earlier pass
// is still live: each run reuses one frame slot.
func (c *borrowCheck) checkViewReuse(in Inst, liveAfter map[LocalID]bool) {
	if in.Op != OpLendBox {
		return
	}
	if c.loopReach == nil {
		c.loopReach = blockReach(c.f.Blocks)
	}
	if !c.loopReach[c.curBlock][c.curBlock] {
		return
	}
	for holder := range liveAfter {
		if holder == in.Dst {
			continue
		}
		for _, o := range c.holders[holder].owners {
			if o.view && o.owner == in.Dst {
				c.reportCode(in.Node, holder, "lent-view-reused")
				return
			}
		}
	}
}

func returnEscapeMessage(b *borrowInfo) string {
	for _, o := range b.owners {
		if o.view {
			return "an `&Interface` view made in this function lives in its frame and cannot be returned; return `&'a T` or take `&'a I` as a parameter"
		}
	}
	return "a borrow cannot be returned"
}

// checkViewSpawn rejects handing a future that holds an `&Interface` view to a call that sees it as a future:
// the call may spawn it, and the task may outlive the frame holding the view.
func (c *borrowCheck) checkViewSpawn(in Inst) {
	if in.Op != OpCall && in.Op != OpCallValue && in.Op != OpFutureNew || in.Ent != 0 && c.p.R.IsTaskRunToCompletion(in.Ent) {
		return
	}
	decl := c.declaredArgTypes(in)
	args := in.Args
	if in.Op != OpCall {
		args = args[1:]
	}
	for _, a := range args {
		t, ok := decl[a]
		if !ok {
			t = c.f.Locals[a].Type
		}
		if !c.holdsAsync(t) {
			continue
		}
		for _, o := range c.holderOwners(a) {
			if o.view {
				c.reportCode(in.Node, a, "lent-view-spawned")
				return
			}
		}
	}
}

// declaredArgTypes maps a call's arguments to their declared parameter types; a bare type parameter cannot be spawned.
func (c *borrowCheck) declaredArgTypes(in Inst) map[LocalID]sem.TypeID {
	if in.Op == OpFutureNew || in.Ent == 0 || c.p.R.Entity(in.Ent).Kind != sem.EntFn {
		return nil
	}
	recvIdx := 0
	if in.Op == OpCallValue {
		recvIdx = 1
	}
	out := map[LocalID]sem.TypeID{}
	for _, s := range c.callSlots(c.p.R.Fn(in.Ent), in.Args, recvIdx) {
		if s.exact != 0 {
			out[s.arg] = s.exact
		}
	}
	return out
}

func (c *borrowCheck) holderOwners(l LocalID) []ownerRef {
	if b, ok := c.holders[l]; ok {
		return b.owners
	}
	return nil
}
