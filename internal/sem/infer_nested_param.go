package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

type nestedFieldKey struct {
	Param EntityID
	Path  []EntityID
	Field EntityID
}

func (r *Result) nestedPath(place EntityID) (EntityID, []EntityID) {
	var path []EntityID
	for r.Entities[place].Kind == EntFieldParam && r.Entities[place].Parent != 0 {
		key, ok := r.fieldKeyByEnt[place]
		if !ok {
			break
		}
		path = append(path, key.Field)
		place = key.Ent
	}
	slices.Reverse(path)
	return place, path
}

func (c *checker) nestedParamPlace(base syntax.NodeID) (EntityID, bool) {
	if len(c.lambdas) > 0 || (c.t.Kind(base) != syntax.MemberExpr && c.t.Kind(base) != syntax.Ident) {
		return 0, false
	}
	ent := c.r.fnChainEnt(c.f, base)
	if ent == 0 || c.r.Entities[ent].Kind != EntFieldParam || c.r.Entities[ent].Parent == 0 {
		return 0, false
	}
	root, _ := c.r.nestedPath(ent)
	e := &c.r.Entities[root]
	if e.Kind != EntParam || e.Parent != c.fn || e.Flags&EfMut != 0 {
		return 0, false
	}
	return ent, true
}

func (c *checker) registerNestedCalled(place, field EntityID) {
	c.r.markDeferred(c.r.fieldParamEnt(fieldFnKey{Ent: place, Field: field}), c.viaCopy)
	param, path := c.r.nestedPath(place)
	key := nestedFieldKey{Param: param, Path: path, Field: field}
	for _, k := range c.r.nestedCalled[c.fn] {
		if k.Param == key.Param && k.Field == key.Field && slices.Equal(k.Path, key.Path) {
			return
		}
	}
	if c.r.nestedCalled == nil {
		c.r.nestedCalled = map[EntityID][]nestedFieldKey{}
	}
	c.r.nestedCalled[c.fn] = append(c.r.nestedCalled[c.fn], key)
}

func (c *checker) nestedFieldCall(n syntax.NodeID, place, field EntityID) {
	c.registerNestedCalled(place, field)
	c.edge(EffectEdge{Kind: EdgeKnownCall, Target: c.r.fieldParamEnt(fieldFnKey{Ent: place, Field: field}), Node: n})
}

func (c *checker) nestedArgEdges(n syntax.NodeID, fn EntityID, args []syntax.NodeID, seen int) {
	keys := c.r.nestedCalled[fn]
	params := c.r.Fn(fn).Params
	for _, key := range keys[min(seen, len(keys)):] {
		i := slices.Index(params, key.Param)
		if i < 0 || i >= len(args) || c.t.Kind(args[i]) == syntax.Spread {
			continue
		}
		owner := key.Param
		for _, f := range key.Path {
			owner = c.r.nestedFnOwner(fieldFnKey{Ent: owner, Field: f})
		}
		dst := c.r.fieldParamEnt(fieldFnKey{Ent: owner, Field: key.Field})
		c.nestedArgBind(n, args[i], key.Path, key.Field, dst)
	}
}

func (c *checker) nestedArgBind(n, arg syntax.NodeID, path []EntityID, field, dst EntityID) {
	arg = c.stripParen(arg)
	for len(path) > 0 && c.t.Kind(arg) == syntax.RecordLit {
		next := c.literalEntry(arg, path[0])
		if next == 0 {
			return
		}
		arg, path = c.stripParen(next), path[1:]
	}
	if c.t.Kind(arg) == syntax.RecordLit {
		if init := c.literalEntry(arg, field); init != 0 {
			c.bindFieldArgValue(n, init, fieldFnKey{Field: field}, dst)
		}
		return
	}
	owner := c.r.fnChainEnt(c.f, arg)
	if owner == 0 || c.r.isLiteralFieldEnt(owner) {
		return
	}
	for _, f := range path {
		owner = c.r.nestedFnOwner(fieldFnKey{Ent: owner, Field: f})
	}
	c.bindIdentFieldArg(n, owner, field, dst)
}

func (c *checker) literalEntry(lit syntax.NodeID, field EntityID) syntax.NodeID {
	for entry := range c.t.View(lit).AsRecordLit().Fields().IDs() {
		if c.t.Kind(entry) != syntax.Spread && c.info.Uses[entry] == field {
			return c.t.View(entry).AsFieldInit().Value().ID()
		}
	}
	return 0
}

func (c *checker) nestedPlaceOfParam(ent EntityID) (EntityID, bool) {
	if len(c.lambdas) > 0 || c.r.Entities[ent].Parent == 0 {
		return 0, false
	}
	root, _ := c.r.nestedPath(ent)
	e := &c.r.Entities[root]
	if e.Kind != EntParam || e.Parent != c.fn || e.Flags&EfMut != 0 {
		return 0, false
	}
	return ent, true
}
