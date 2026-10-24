package mir

import (
	"slices"
	"strings"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// mergeInto folds src's borrows into dst and reports whether dst grew; local owners absorb
// a parameter's borrow, and two different parameter lifetimes merge to 0, which no return matches.
func mergeInto(dst, src *borrowInfo) bool {
	if src == nil || dst == src || src.empty {
		return false
	}
	if dst.empty {
		dst.empty = false
		dst.hasOwner, dst.lifetime, dst.fromStatic, dst.multi = src.hasOwner, src.lifetime, src.fromStatic, src.multi
		dst.owners = append([]ownerRef(nil), src.owners...)
		return true
	}
	changed := false
	if dst.multi == 0 && src.multi != 0 {
		dst.multi, changed = src.multi, true
	}
	switch {
	case src.hasOwner:
		if !dst.hasOwner {
			dst.hasOwner, dst.lifetime, dst.fromStatic, changed = true, 0, false, true
		}
		for _, o := range src.owners {
			if !slices.ContainsFunc(dst.owners, o.same) {
				dst.owners = append(dst.owners, o)
				changed = true
			}
		}
	case dst.hasOwner || src.fromStatic:
	case dst.fromStatic:
		dst.lifetime, dst.fromStatic, changed = src.lifetime, false, true
	case dst.lifetime != src.lifetime && dst.lifetime != 0:
		dst.lifetime, changed = 0, true
	}
	return changed
}

func (o ownerRef) same(p ownerRef) bool {
	return o.owner == p.owner && o.mut == p.mut && o.exact == p.exact && o.view == p.view && o.carried == p.carried && o.cell == p.cell && o.lifetime == p.lifetime && slices.Equal(o.path, p.path)
}

// sink is l's re-merged holder, created on first use from what l already holds.
func (c *borrowCheck) sink(l LocalID, node syntax.NodeID) *borrowInfo {
	if s, ok := c.sinks[l]; ok {
		return s
	}
	s := &borrowInfo{empty: true, node: node}
	if old, ok := c.holders[l]; ok {
		mergeInto(s, old)
		s.node = old.node
	}
	c.sinks[l] = s
	c.holders[l] = s
	c.grew = true
	return s
}

// soleLifetime is the one named lifetime reachable through t's generic
// arguments, 0 for none or for more than one.
func (c *borrowCheck) soleLifetime(t sem.TypeID) sem.EntityID {
	var sole sem.EntityID
	for _, p := range c.p.R.Types.Params(t) {
		if !c.p.R.TypeParam(p).IsLifetime || p == sole {
			continue
		}
		if sole != 0 {
			return 0
		}
		sole = p
	}
	return sole
}

// multiLifetime reports whether t names more than one distinct lifetime.
func (c *borrowCheck) multiLifetime(t sem.TypeID) bool {
	var first sem.EntityID
	for _, p := range c.p.R.Types.Params(t) {
		if !c.p.R.TypeParam(p).IsLifetime || p == first {
			continue
		}
		if first != 0 {
			return true
		}
		first = p
	}
	return false
}

// elementHolder reports whether a value of type t may sit in a tracked
// container: it names a lifetime and is not itself a bare reference.
func (c *borrowCheck) elementHolder(t sem.TypeID) bool {
	return c.p.R.Types.Kind(t) != sem.KRef && c.declaredLifetime(t) != 0
}

// trackedContainer reports whether collect follows t's element sources: an array, fixed array,
// Option, Result, tuple, std Map, or a record or variant declared outside std.
func (c *borrowCheck) trackedContainer(t sem.TypeID) bool {
	if c.p.R.Types.Kind(t) != sem.KNamed {
		return false
	}
	tt := c.p.R.Types
	ent := tt.Node(t).Ent
	if ent == tt.ArrayEnt() || ent == tt.OptionEnt() || ent == tt.ResultEnt() || tt.IsTupleEnt(ent) || c.p.R.IsFixedArray(t) {
		return true
	}
	if path := c.p.R.Packages[c.p.R.Entity(ent).Pkg].Path; strings.HasPrefix(path, "std/") {
		return path == "std/map" && c.p.R.Entity(ent).Name == "Map"
	}
	switch c.p.R.TypeDecl(ent).Form {
	case sem.FormRecord, sem.FormAdt, sem.FormDefined:
		return true
	}
	return false
}

func (c *borrowCheck) multiDefined(l LocalID) bool {
	c.definition(l)
	d, ok := c.defs[l]
	return ok && d == nil
}
