package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// A mutable local copied from a by-value parameter's place still holds what callers bind to that place, besides its own writes.
func (c *checker) notePlaceCopy(local, src EntityID) bool {
	if src == 0 || local == src || !c.paramPlaceRoot(src) {
		return false
	}
	if !slices.Contains(c.r.flow.paramCopy[local], src) {
		if c.r.flow.paramCopy == nil {
			c.r.flow.paramCopy = map[EntityID][]EntityID{}
		}
		c.r.flow.paramCopy[local] = append(c.r.flow.paramCopy[local], src)
	}
	return true
}

func (c *checker) paramPlaceRoot(ent EntityID) bool {
	if len(c.lambdas) > 0 {
		return false
	}
	root, _ := c.r.nestedPath(ent)
	e := &c.r.Entities[root]
	return e.Kind == EntParam && e.Parent == c.fn && e.Flags&EfMut == 0
}

func (r *Result) paramPlacesOf(ent EntityID) []EntityID {
	if ent == 0 {
		return nil
	}
	root, path := r.nestedPath(ent)
	var out []EntityID
	for _, src := range r.flow.paramCopy[root] {
		owner := src
		for _, f := range path {
			owner = r.nestedFnOwner(fieldFnKey{Ent: owner, Field: f})
		}
		out = append(out, owner)
	}
	return out
}

func (c *checker) copiedPlaceCalls(n, base syntax.NodeID, field EntityID) {
	if base == 0 || c.t.Kind(base) != syntax.Ident && c.t.Kind(base) != syntax.MemberExpr {
		return
	}
	c.viaCopy = true
	defer func() { c.viaCopy = false }()
	for _, place := range c.r.paramPlacesOf(c.r.fnChainEnt(c.f, base)) {
		if c.r.Entities[place].Kind == EntParam {
			c.deferToFieldParam(place, field)
			c.edge(EffectEdge{Kind: EdgeKnownCall, Target: c.r.fieldParamEnt(fieldFnKey{Ent: place, Field: field}), Node: n})
			continue
		}
		c.nestedFieldCall(n, place, field)
	}
}

// c.fn hands its parameter's field to a callee that calls it, so its callers must bind that field.
func (c *checker) noteFwdCalled(param, field EntityID) {
	c.r.markDeferred(c.r.fieldParamEnt(fieldFnKey{Ent: param, Field: field}), false)
	if c.fn == 0 || len(c.lambdas) > 0 || c.r.Entities[param].Parent != c.fn || c.r.Entities[param].Flags&EfMut != 0 {
		return
	}
	key := FieldParamKey{Param: param, Field: field}
	if slices.Contains(c.r.Fn(c.fn).CalledFieldParams, key) || slices.Contains(c.r.flow.fwdCalled[c.fn], key) {
		return
	}
	if c.r.flow.fwdCalled == nil {
		c.r.flow.fwdCalled = map[EntityID][]FieldParamKey{}
	}
	c.r.flow.fwdCalled[c.fn] = append(c.r.flow.fwdCalled[c.fn], key)
}
