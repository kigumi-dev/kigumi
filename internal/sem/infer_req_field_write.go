package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

func (c *checker) recordReqFieldCallSite(n syntax.NodeID, req EntityID, base syntax.NodeID, args []syntax.NodeID) {
	recv := c.r.chainFieldEnt(c.f, base)
	if recv == 0 {
		return
	}
	site := methodFieldCallSite{file: c.f, node: n, caller: c.fn, recv: recv, req: req, args: args}
	c.r.methodFieldCalls = append(c.r.methodFieldCalls, site)
	c.r.applyMethodFieldWrites(site)
}

func (c *checker) recordBorrowFieldCallSites(n syntax.NodeID, fn EntityID, args []syntax.NodeID) {
	info := c.r.Fn(fn)
	for i, param := range info.Params {
		if i >= len(args) {
			continue
		}
		owner, ok := c.r.recordParamOwner(param)
		if !ok {
			continue
		}
		recv := c.r.chainFieldEnt(c.f, args[i])
		if recv == 0 {
			continue
		}
		site := methodFieldCallSite{file: c.f, node: n, caller: c.fn, recv: recv, callee: fn, srcParam: param, owner: owner, args: args}
		c.r.methodFieldCalls = append(c.r.methodFieldCalls, site)
		c.r.applyMethodFieldWrites(site)
	}
}

func (r *Result) recordParamOwner(param EntityID) (EntityID, bool) {
	t := r.Entities[param].Type
	if !r.Types.RefMut(t) {
		return 0, false
	}
	elem := r.Types.Node(t).Elem
	switch r.Types.Kind(elem) {
	case KNamed:
		return r.Types.Node(elem).Ent, true
	case KParam, KIface:
		return 0, true
	}
	return 0, false
}

func (r *Result) writtenFields(ent EntityID) []EntityID {
	var fields []EntityID
	add := func(k fieldFnKey) {
		if k.Ent == ent && !slices.Contains(fields, k.Field) {
			fields = append(fields, k.Field)
		}
	}
	for k := range r.knownFieldFn {
		add(k)
	}
	for k := range r.fieldWitnessPoisoned {
		add(k)
	}
	for k := range r.fieldParamEnts {
		add(k)
	}
	for k := range r.fieldWitnessOf {
		add(k)
	}
	slices.Sort(fields)
	return fields
}
