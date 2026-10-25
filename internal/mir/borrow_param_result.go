package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// frameBorrows is the part of b that borrows this frame's own storage, nil for none; a borrow of what a
// reference parameter or a `self` receiver reaches is the caller's value, which outlives the call.
func (c *borrowCheck) frameBorrows(b *borrowInfo) *borrowInfo {
	if !b.hasOwner {
		return nil
	}
	owners := slices.DeleteFunc(slices.Clone(b.owners), func(o ownerRef) bool { return c.callerStorage(o.owner) })
	switch len(owners) {
	case 0:
		return nil
	case len(b.owners):
		return b
	}
	kept := *b
	kept.owners = owners
	return &kept
}

// valueResultSource makes a value or witness call whose result names a type parameter a carrier of the borrows its
// arguments take of this frame: the signature cannot say the result does not keep them.
func (c *borrowCheck) valueResultSource(in Inst) (*borrowInfo, bool) {
	if !c.namesTypeParam(c.f.Locals[in.Dst].Type) {
		return nil, false
	}
	var found []*borrowInfo
	whole := true
	for _, a := range in.Args[1:] {
		h, ok := c.holders[a]
		if !ok {
			continue
		}
		if b := c.carriedBorrows(h); b != nil && !slices.Contains(found, b) {
			found = append(found, b)
			whole = whole && b == h
		}
	}
	switch {
	case len(found) == 0:
		return nil, false
	case len(found) == 1 && whole:
		return found[0], true
	}
	s := c.sink(in.Dst, in.Node)
	for _, b := range found {
		c.merge(s, b)
	}
	return s, true
}

// projectsOwned reports whether a read of type t out of src keeps a borrow src takes of this frame through a type parameter.
func (c *borrowCheck) projectsOwned(t sem.TypeID, src LocalID) bool {
	b, ok := c.holders[src]
	return ok && c.namesTypeParam(t) && c.frameBorrows(b) != nil
}

// storedInfo is what v puts into a store; a flow found only through a type parameter stores this frame's borrows,
// and into this frame's own place also the marked borrows of a caller place the body may write.
func (c *borrowCheck) storedInfo(fl storeFlow, v flowArg, caller bool) (*borrowInfo, bool) {
	b, ok := c.tieInfo(v, fl.generic, caller)
	if !ok || !fl.generic {
		return b, ok
	}
	if caller {
		b = c.frameBorrows(b)
	} else {
		b = c.carriedBorrows(b)
	}
	return b, b != nil
}

// checkCallerStore rejects writing a value that borrows this frame through a `&mut` parameter or a receiver, which outlive the call.
func (c *borrowCheck) checkCallerStore(in Inst) {
	v := in.Args[len(in.Args)-1]
	b, ok := c.holders[v]
	if !ok || c.frameBorrows(b) == nil {
		return
	}
	if root, _, caller, ok := c.storeRoot(storeFlow{target: in.Args[0], ref: true}); ok && caller {
		c.report(in.Node, v, "borrow-escape", "a borrow of a value this function owns cannot be stored through `"+c.f.Locals[root].Name+"`, which outlives the call")
	}
}

// carriedBorrows is what a value call's result keeps of b: this frame's borrows, and borrows of a caller place the body
// may write, marked caller so they only conflict with writes here and are not an escape on return.
func (c *borrowCheck) carriedBorrows(b *borrowInfo) *borrowInfo {
	if !b.hasOwner {
		return nil
	}
	var owners []ownerRef
	for _, o := range b.owners {
		var root []LocalID
		switch {
		case !c.callerStorageFrom(o.owner, false, 0, &root):
			owners = append(owners, o)
		case len(root) == 1 && c.writableCaller(root[0]):
			o.caller = true
			owners = append(owners, o)
		}
	}
	switch {
	case len(owners) == 0:
		return nil
	case slices.EqualFunc(owners, b.owners, ownerRef.same):
		return b
	}
	kept := *b
	kept.owners = owners
	return &kept
}

func (c *borrowCheck) writableCaller(l LocalID) bool {
	t := c.f.Locals[l].Type
	return c.p.R.Types.Kind(t) == sem.KRef && c.p.R.Types.RefMut(t) || c.f.Locals[l].Ent != 0 && c.p.R.IsMutSelf(c.f.Locals[l].Ent)
}

// callerKept reports whether every borrow b holds is a caller place kept only for conflicts.
func (c *borrowCheck) callerKept(b *borrowInfo) bool {
	return b.hasOwner && len(b.owners) > 0 && !slices.ContainsFunc(b.owners, func(o ownerRef) bool { return !o.caller })
}
