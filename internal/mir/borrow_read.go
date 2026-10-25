package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// readInfo is the holder a value read out of src (a field, element or
// payload) shares: what src holds, or a lent parameter's contents.
func (c *borrowCheck) readInfo(src LocalID) (*borrowInfo, bool) {
	if b, ok := c.paramContents(src); ok {
		return b, true
	}
	b, ok := c.holders[src]
	return b, ok
}

// paramContents is what an untagged `&X` parameter's referent holds when X names a lifetime:
// the reference stays function-scoped, but a value read through it carries X's lifetime.
func (c *borrowCheck) paramContents(l LocalID) (*borrowInfo, bool) {
	if b, ok := c.contents[l]; ok {
		return b, true
	}
	t := c.f.Locals[l].Type
	if c.p.R.Types.Kind(t) != sem.KRef || !slices.Contains(c.f.Params, l) {
		return nil, false
	}
	b, ok := c.holders[l]
	if !ok || b.hasOwner || b.lifetime != 0 || b.fromStatic {
		return nil, false
	}
	lt := c.soleLifetime(c.p.R.Types.Node(t).Elem)
	if lt == 0 {
		return nil, false
	}
	info := &borrowInfo{lifetime: lt, node: b.node}
	c.contents[l] = info
	return info, true
}

// argContents is what a reference argument's referent holds: the borrowed root's holder, a lent
// parameter's contents, or the reference's own holder when neither can be traced.
func (c *borrowCheck) argContents(a LocalID) (*borrowInfo, bool) {
	for {
		if b, ok := c.paramContents(a); ok {
			return b, true
		}
		if c.f.Locals[a].Ent != 0 {
			break
		}
		in := c.definition(a)
		if in == nil || len(in.Args) == 0 {
			break
		}
		if in.Op == OpBorrow {
			root, _, _, ok := c.projectionPlace(in.Args[0])
			if !ok {
				break
			}
			if b, ok := c.paramContents(root); ok {
				return b, true
			}
			b, ok := c.holders[root]
			return b, ok
		}
		if in.Op != OpNewCell && in.Op != OpShare && in.Op != OpCopy && in.Op != OpMove && in.Op != OpAlias {
			break
		}
		a = in.Args[0]
	}
	b, ok := c.holders[a]
	return b, ok
}

// flowArg is one argument whose value may flow into a call's result or a
// store target; contents means the value is behind the argument's reference.
type flowArg struct {
	arg      LocalID
	contents bool
	tie      tieKind
	shared   bool
}

// flowInfoOf is what one flowing argument contributes: an immediate closure
// contributes what it captured, since its results can only come from there.
func (c *borrowCheck) flowInfoOf(f flowArg) (*borrowInfo, bool) {
	if def := c.definition(f.arg); def != nil && def.Op == OpClosure && c.p.R.Entity(def.Ent).Kind == sem.EntClosure {
		s := c.captureInfo(f.arg, def)
		return s, !s.empty
	}
	if f.contents {
		return c.argContents(f.arg)
	}
	b, ok := c.holders[f.arg]
	return b, ok
}

func (c *borrowCheck) captureInfo(l LocalID, def *Inst) *borrowInfo {
	s, ok := c.caps[l]
	if !ok {
		s = &borrowInfo{empty: true, node: def.Node}
		c.caps[l] = s
	}
	for _, a := range def.Args {
		if b, ok := c.holders[a]; ok {
			c.merge(s, b)
		}
		if c.p.R.Types.Kind(c.f.Locals[a].Type) != sem.KRef {
			continue
		}
		if b, ok := c.argContents(a); ok {
			c.merge(s, b)
		}
	}
	c.merge(s, c.deferredCaptures(def))
	c.merge(s, c.returnedCaptures(def))
	return s
}

// deferredCaptures is what a closure returning a Future or a Task keeps borrowed:
// the locals it captures without owning, which the value it returns may still reach.
func (c *borrowCheck) deferredCaptures(def *Inst) *borrowInfo {
	info := c.p.R.Closure(def.Ent)
	if !c.holdsAsync(info.Ret) || len(info.Captures) != len(def.Args) {
		return nil
	}
	out := &borrowInfo{hasOwner: true, node: def.Node}
	for i, cp := range info.Captures {
		if cp.Mode == sem.CapCopy || cp.Mode == sem.CapMove || cp.Mode == sem.CapRetain {
			continue
		}
		if root, path, exact, ok := c.projectionPlace(def.Args[i]); ok {
			out.owners = append(out.owners, ownerRef{owner: root, mut: cp.Mode != sem.CapRead, path: path, exact: exact})
		}
	}
	if len(out.owners) == 0 {
		return nil
	}
	return out
}
