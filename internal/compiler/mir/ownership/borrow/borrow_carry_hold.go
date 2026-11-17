package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// mayCarry reports whether a result of type ret can keep an argument of type arg inside it.
func (c *borrowCheck) mayCarry(ret, arg sem.TypeID) bool {
	return c.p.R.Types.Contains(ret, arg) || c.holdsDeferred(arg) && c.holdsDeferred(ret)
}

// slotCarry is what one argument passes on to a result of type ret: a by-value argument the result keeps,
// or the holders of a receiver's or reference's referent when the result can take a Future, Task or closure out.
func (c *borrowCheck) slotCarry(s callSlot, ret sem.TypeID) []*borrowInfo {
	t := c.f.Locals[s.arg].Type
	if s.exact != 0 && c.p.R.Types.Kind(s.exact) != sem.KRef {
		if c.resultMayCarryLifetime(t) || !c.mayCarry(ret, t) {
			return nil
		}
		if b, ok := c.carriedArg(s.arg); ok {
			return []*borrowInfo{b}
		}
		return nil
	}
	for c.p.R.Types.Kind(t) == sem.KRef {
		t = c.p.R.Types.Node(t).Elem
	}
	if c.resultMayCarryLifetime(t) || !c.holdsDeferred(t) || !c.holdsDeferred(ret) {
		return nil
	}
	return c.referentHolders(s.arg)
}

// referentHolders are the holders of what a receiver or reference argument reaches; a
// reference parameter's referent belongs to the caller, which tracks it at its own call.
func (c *borrowCheck) referentHolders(a midir.LocalID) []*borrowInfo {
	l := c.f.Locals[a]
	if c.p.R.Types.Kind(l.Type) != sem.KRef {
		if b, ok := c.holders[a]; ok {
			return []*borrowInfo{b}
		}
		return nil
	}
	if l.Ent != 0 {
		if b, ok := c.paramContents(a); ok {
			return []*borrowInfo{b}
		}
		ref, ok := c.holders[a]
		if !ok {
			return nil
		}
		var out []*borrowInfo
		for _, o := range ref.owners {
			if b, ok := c.holders[o.owner]; ok && c.p.R.Types.Kind(c.f.Locals[o.owner].Type) != sem.KRef {
				out = append(out, b)
			}
		}
		return out
	}
	in := c.definition(a)
	if in == nil || len(in.Args) == 0 {
		return nil
	}
	switch in.Op {
	case midir.OpBorrow:
		if root, _, _, ok := c.projectionPlace(in.Args[0]); ok {
			return c.referentHolders(root)
		}
		return c.referentHolders(in.Args[0])
	case midir.OpNewCell, midir.OpShare, midir.OpCopy, midir.OpMove, midir.OpAlias:
		return c.referentHolders(in.Args[0])
	}
	return nil
}

// projectsHeld reports whether a value read out of in's source keeps what the source holds: it names a lifetime,
// a type parameter while the source borrows this frame, or a Future, Task or closure that may move out of an owned source.
func (c *borrowCheck) projectsHeld(in midir.Inst) bool {
	t := c.f.Locals[in.Dst].Type
	if c.resultMayCarryLifetime(t) || c.projectsOwned(t, in.Args[0]) {
		return true
	}
	return c.holdsDeferred(t) && (in.Op == midir.OpFieldMove || c.p.R.Types.Kind(c.f.Locals[in.Args[0]].Type) != sem.KRef)
}

// holdsDeferred reports whether a value of type t can keep a Future, a Task or a capture
// inside it: through a type argument, an element, or a field or payload of a named type.
func (c *borrowCheck) holdsDeferred(t sem.TypeID) bool {
	return c.typeHolds(t, func(t sem.TypeID) bool {
		n := c.p.R.Types.Node(t)
		return n.Kind == sem.KIface || n.Kind == sem.KClosure || n.Kind == sem.KNamed && (n.Ent == c.p.R.Types.FutureEnt() || c.p.R.IsTaskType(t))
	})
}

// holdsAsync reports whether a value of type t can keep a Future or a Task inside it.
func (c *borrowCheck) holdsAsync(t sem.TypeID) bool {
	return c.typeHolds(t, func(t sem.TypeID) bool {
		return c.p.R.Types.Kind(t) == sem.KNamed && c.p.R.Types.Node(t).Ent == c.p.R.Types.FutureEnt() || c.p.R.IsTaskType(t)
	})
}

// releaseOnly reports whether dropping a holder named l runs nothing that reaches what it borrows:
// it only carries borrows inside values that run solely when called or awaited, never a Task.
func (c *borrowCheck) releaseOnly(l midir.LocalID) bool {
	b, ok := c.holders[l]
	if !ok || len(b.owners) == 0 || c.f.Locals[l].Ent == 0 {
		return false
	}
	for _, o := range b.owners {
		if !o.carried {
			return false
		}
	}
	return !c.typeHolds(c.f.Locals[l].Type, func(t sem.TypeID) bool {
		return c.p.R.Types.Kind(t) == sem.KIface || c.p.R.IsTaskType(t)
	})
}
