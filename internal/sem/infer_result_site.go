package sem

import "kigumi/internal/syntax"

func (r *Result) resultRecordEnt(t TypeID) EntityID {
	if r.Types.Kind(t) != KNamed || !r.holdsFn(t, 0) {
		return 0
	}
	ent := r.Types.Node(t).Ent
	if r.Entities[ent].Kind != EntType || len(r.typeDecl(ent).Fields) == 0 {
		return 0
	}
	return ent
}

func (r *Result) fnResultEnt(fn EntityID) EntityID {
	if id, ok := r.fnResultEnts[fn]; ok {
		return id
	}
	e := &r.Entities[fn]
	id := r.newEntity(Entity{Kind: EntFieldParam, Name: "<result>", Pkg: e.Pkg, File: e.File, Node: e.Node, Flags: EfResult})
	if r.fnResultEnts == nil {
		r.fnResultEnts = map[EntityID]EntityID{}
	}
	r.fnResultEnts[fn] = id
	return id
}

func (r *Result) callResultEnt(file FileID, call syntax.NodeID, fn EntityID) EntityID {
	key := literalEntKey{File: file, Node: call}
	if id, ok := r.resultEnts[key]; ok {
		return id
	}
	id := r.newEntity(Entity{Kind: EntFieldParam, Name: "<call>", Pkg: r.Files[file].Pkg, File: file, Node: call, Flags: EfResult, Target: fn})
	if r.resultEnts == nil {
		r.resultEnts = map[literalEntKey]EntityID{}
	}
	r.resultEnts[key] = id
	return id
}

func (r *Result) isResultEnt(ent EntityID) bool { return r.Entities[ent].Flags&EfResult != 0 }

// Replayed like a setter's writes, so the order the bodies are checked in never matters.
func (c *checker) noteResultSite(n syntax.NodeID, fn EntityID, args []syntax.NodeID) {
	info := c.r.Fn(fn)
	if info.Body == 0 || info.Declared&EffAsync != 0 {
		return
	}
	owner := c.r.resultRecordEnt(c.r.Types.Node(info.Sig).Elem)
	if owner == 0 {
		return
	}
	site := methodFieldCallSite{file: c.f, node: n, caller: c.fn, recv: c.r.callResultEnt(c.f, n, fn), callee: fn,
		srcParam: c.r.fnResultEnt(fn), owner: owner, args: args}
	c.r.methodFieldCalls = append(c.r.methodFieldCalls, site)
	c.r.applyMethodFieldWrites(site)
}

func (c *checker) recordResultLeaves(body syntax.NodeID) {
	c.recordFnResultLeaves(body)
	if c.r.resultRecordEnt(c.retType) == 0 {
		return
	}
	ret := c.r.fnResultEnt(c.fn)
	for _, leaf := range c.resultLeaves(body) {
		if lit := c.stripParen(leaf); c.t.Kind(lit) == syntax.RecordLit {
			c.recordKnownFieldFns(lit, ret)
		} else if src := c.r.fnChainEnt(c.f, leaf); src != 0 && c.r.concretePlace(src) {
			c.unionInto(ret, src, c.retType, leaf)
		}
	}
}
