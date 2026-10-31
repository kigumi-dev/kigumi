package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) registerContainerFlows(body syntax.NodeID) {
	var walk func(n syntax.NodeID)
	walk = func(n syntax.NodeID) {
		if n == 0 {
			return
		}
		c.fillContainerLit(n)
		for ch := range c.t.View(n).Children() {
			walk(ch.ID())
		}
	}
	walk(body)
}

func (c *checker) fillContainerLit(n syntax.NodeID) {
	elems, ok := c.r.containerLitElems(c.f, n)
	if !ok {
		return
	}
	fields, types, ok := c.r.containerShape(c.vars.resolve(c.info.Types[n]))
	if !ok {
		return
	}
	ent := c.r.containerLitEnt(c.f, n)
	for i, e := range elems {
		k := c.r.containerLitSlot(c.f, n, i, len(fields))
		if c.r.holdsFn(types[k], 0) || c.r.holdsFnElem(types[k]) {
			c.flowComponent(c.r.nestedFnOwner(fieldFnKey{Ent: ent, Field: fields[k]}), types[k], e)
		}
	}
}

func (c *checker) flowComponent(place EntityID, t TypeID, e syntax.NodeID) {
	e = c.stripParen(e)
	switch {
	case c.t.Kind(e) == syntax.RecordLit:
		c.recordKnownFieldFns(e, place)
	default:
		if src := c.r.fnChainEnt(c.f, e); src != 0 && c.r.concretePlace(src) {
			c.flowPlace(place, src, t, e)
		}
	}
}

func (c *checker) flowPlace(dst, src EntityID, t TypeID, node syntax.NodeID) {
	t = c.vars.resolve(t)
	fields, elems, ok := c.r.containerShape(t)
	if !ok {
		c.unionInto(dst, src, t, node)
		return
	}
	for i, f := range fields {
		if c.r.holdsFn(elems[i], 0) || c.r.holdsFnElem(elems[i]) {
			c.flowPlace(c.r.nestedFnOwner(fieldFnKey{Ent: dst, Field: f}), c.r.nestedFnOwner(fieldFnKey{Ent: src, Field: f}), elems[i], node)
		}
	}
}

func (c *checker) recordContainerResult(body syntax.NodeID) {
	ret := c.r.fnResultEnt(c.fn)
	for _, leaf := range c.expandLetLeaves(c.resultLeaves(body), 0) {
		c.recordResultElems(ret, nil, c.retType, c.stripParen(leaf))
	}
}

// recordResultElems follows a container literal returned at path below the result: a param stays a call-site flow, anything else is recorded here.
func (c *checker) recordResultElems(ret EntityID, path []EntityID, t TypeID, lit syntax.NodeID) {
	elems, ok := c.r.containerLitElems(c.f, lit)
	fields, types, shaped := c.r.containerShape(t)
	if !ok || !shaped || len(path) >= maxNestedFnDepth {
		return
	}
	for i, e := range elems {
		k := c.r.containerLitSlot(c.f, lit, i, len(fields))
		here := append(append([]EntityID(nil), path...), fields[k])
		e = c.stripParen(e)
		if _, _, nested := c.r.containerShape(types[k]); nested {
			c.recordResultElems(ret, here, types[k], e)
			continue
		}
		if !c.r.holdsFn(types[k], 0) {
			continue
		}
		if src := c.r.fnChainEnt(c.f, e); src != 0 && !c.r.concretePlace(src) {
			c.noteResultParamAt(here, src, e)
			continue
		}
		place := ret
		for _, f := range here {
			place = c.r.nestedFnOwner(fieldFnKey{Ent: place, Field: f})
		}
		c.flowComponent(place, types[k], e)
	}
}

// containerResultSite stages a call whose result is a tuple, Option or Array holding fns, so the callee's component facts reach the call's result.
func (c *checker) containerResultSite(n syntax.NodeID, fn EntityID, args []syntax.NodeID, recvBase syntax.NodeID, ret TypeID) {
	if !c.r.holdsFnElem(ret) {
		return
	}
	site := methodFieldCallSite{file: c.f, node: n, caller: c.fn, recv: c.r.callResultEnt(c.f, n, fn), callee: fn,
		srcParam: c.r.fnResultEnt(fn), args: args, recvNode: recvBase, retType: ret}
	c.r.methodFieldCalls = append(c.r.methodFieldCalls, site)
	c.r.applyMethodFieldWrites(site)
}

func (r *Result) applyComponentSummary(site methodFieldCallSite, t TypeID, srcParam EntityID, depth int) bool {
	fields, elems, ok := r.containerShape(t)
	if !ok || depth >= maxNestedFnDepth {
		return false
	}
	changed := false
	for i, f := range fields {
		inner := site
		inner.recv = r.nestedFnOwner(fieldFnKey{Ent: site.recv, Field: f})
		src := r.nestedFnOwner(fieldFnKey{Ent: srcParam, Field: f})
		if _, _, nested := r.containerShape(elems[i]); nested {
			if r.applyComponentSummary(inner, elems[i], src, depth+1) {
				changed = true
			}
			continue
		}
		for _, rf := range r.recordFieldsOf(elems[i]) {
			if r.applyFieldWriteSummary(inner, rf, site.callee, src) {
				changed = true
			}
		}
	}
	return changed
}

func (c *checker) flowContainerField(key fieldFnKey, t TypeID, init syntax.NodeID) bool {
	src := c.r.fnChainEnt(c.f, init)
	if src == 0 || !c.r.concretePlace(src) {
		return false
	}
	c.flowPlace(c.r.nestedFnOwner(key), src, t, init)
	return true
}
