package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

func (c *checker) trackPatternPlaces(p syntax.NodeID, place EntityID) {
	if place == 0 || p == 0 {
		return
	}
	t := c.vars.resolve(c.info.Types[p])
	switch c.t.Kind(p) {
	case syntax.PatBind:
		local := c.info.Defs[p]
		if local == 0 || c.r.Entities[local].Flags&EfMut != 0 || place == local {
			return
		}
		if !c.r.holdsFn(t, 0) && !c.r.holdsFnElem(t) {
			return
		}
		if _, taken := c.r.fnCopyAlias[local]; !taken {
			if c.r.fnCopyAlias == nil {
				c.r.fnCopyAlias = map[EntityID]EntityID{}
			}
			c.r.fnCopyAlias[local] = place
		}
	case syntax.PatTuple:
		c.trackComponentPlaces(t, slices.Collect(c.t.List(p).IDs()), place)
	case syntax.PatCtor:
		if v := c.info.Uses[p]; v != 0 && c.r.Entities[v].Kind == EntVariant && c.r.Entities[v].Parent == c.r.Types.optionEnt {
			c.trackComponentPlaces(t, slices.Collect(c.t.View(p).AsPatCtor().Args().IDs()), place)
		}
	}
}

func (c *checker) trackComponentPlaces(t TypeID, subs []syntax.NodeID, place EntityID) {
	fields, _, ok := c.r.containerShape(t)
	if !ok {
		return
	}
	for i, s := range subs {
		if i < len(fields) {
			c.trackPatternPlaces(s, c.r.nestedFnOwner(fieldFnKey{Ent: place, Field: fields[i]}))
		}
	}
}
