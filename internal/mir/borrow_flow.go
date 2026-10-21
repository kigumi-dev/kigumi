package mir

import (
	"slices"

	"kigumi/internal/sem"
)

type callSlot struct {
	arg      LocalID
	params   []sem.EntityID
	target   bool
	contents bool
	exact    sem.TypeID
}

func (c *borrowCheck) callSlots(info *sem.FnInfo, args []LocalID, recvIdx int) []callSlot {
	var out []callSlot
	start := recvIdx
	if info.Recv != sem.RecvNone {
		start++
		if recvIdx < len(args) {
			out = append(out, callSlot{arg: args[recvIdx], params: c.recvLifetimeParams(info), target: info.Recv == sem.RecvMut, contents: info.Recv != sem.RecvMove})
		}
	}
	for i, prm := range info.Params {
		if start+i >= len(args) {
			break
		}
		decl := c.p.R.Entity(prm).Type
		ref := c.p.R.Types.Kind(decl) == sem.KRef
		params := c.p.R.Types.Params(decl)
		if lt := c.p.R.Local(prm).Lifetime; lt != 0 {
			params = append(params, lt)
		}
		out = append(out, callSlot{arg: args[start+i], params: params, target: ref && c.p.R.Types.RefMut(decl), contents: ref && c.p.R.Local(prm).Lifetime == 0, exact: decl})
	}
	return out
}

func sharesParam(a, b []sem.EntityID) bool {
	return slices.ContainsFunc(a, func(p sem.EntityID) bool { return slices.Contains(b, p) })
}

// callFlows lists, for each `&mut` / `mut self` argument whose place names a
// lifetime, the other arguments whose declared types share a parameter with it.
func (c *borrowCheck) callFlows(in Inst) []storeFlow {
	if in.Ent == 0 || c.p.R.Entity(in.Ent).Kind != sem.EntFn {
		return nil
	}
	info := c.p.R.Fn(in.Ent)
	if info.Recv != sem.RecvMut && !slices.ContainsFunc(info.Params, func(p sem.EntityID) bool {
		return c.p.R.Types.RefMut(c.p.R.Entity(p).Type)
	}) {
		return nil
	}
	recvIdx := 0
	if in.Op == OpCallValue {
		recvIdx = 1
	}
	slots := c.callSlots(info, in.Args, recvIdx)
	c.linkSlots(info, slots)
	var out []storeFlow
	for i, t := range slots {
		if !t.target || c.declaredLifetime(c.f.Locals[t.arg].Type) == 0 && c.declaredLifetime(c.placeOf(t.arg)) == 0 {
			continue
		}
		fl := storeFlow{target: t.arg, ref: true}
		for j, v := range slots {
			if i != j && sharesParam(t.params, v.params) {
				fl.values = append(fl.values, flowArg{arg: v.arg, contents: v.contents})
			}
		}
		if fl.values != nil {
			out = append(out, fl)
		}
	}
	return out
}

func (c *borrowCheck) placeOf(l LocalID) sem.TypeID {
	if _, place, _, ok := c.storeRoot(storeFlow{target: l, ref: true}); ok {
		return place
	}
	return 0
}

// genericFlowSource ties a generic call's result to every argument whose declared type names a
// type parameter the result names, widened through callbacks returning one: only those can build it.
func (c *borrowCheck) genericFlowSource(in Inst, recvIdx int) (*borrowInfo, bool) {
	info := c.p.R.Fn(in.Ent)
	if c.fnRetLifetime(info) != 0 || !c.resultMayCarryLifetime(c.f.Locals[in.Dst].Type) {
		return nil, false
	}
	ret := c.p.R.Types.Node(info.Sig).Elem
	bare := c.p.R.Types.Kind(ret) == sem.KParam
	if !bare && !c.trackedContainer(c.f.Locals[in.Dst].Type) {
		return nil, false
	}
	want := c.typeParamsOf(ret)
	if len(want) == 0 {
		return nil, false
	}
	slots := c.callSlots(info, in.Args, recvIdx)
	c.linkSlots(info, slots)
	for grown := true; grown; {
		grown = false
		for _, s := range slots {
			if c.p.R.Types.Kind(s.exact) != sem.KFn {
				continue
			}
			fn := c.p.R.Types.Node(s.exact)
			if !sharesParam(c.typeParamsOf(fn.Elem), want) {
				continue
			}
			for _, a := range fn.Args {
				for _, p := range c.typeParamsOf(a) {
					if !slices.Contains(want, p) {
						want, grown = append(want, p), true
					}
				}
			}
		}
	}
	var srcs []*borrowInfo
	extra := false
	for _, s := range slots {
		if !sharesParam(s.params, want) {
			continue
		}
		if b, ok := c.flowInfo(flowArg{arg: s.arg, contents: s.contents}); ok {
			srcs = append(srcs, b)
			extra = extra || !bare || s.exact != ret
		}
	}
	if bare && !extra {
		return nil, false
	}
	s := c.sink(in.Dst, in.Node)
	for _, b := range srcs {
		c.merge(s, b)
	}
	return s, true
}

func (c *borrowCheck) typeParamsOf(t sem.TypeID) []sem.EntityID {
	var out []sem.EntityID
	for _, p := range c.p.R.Types.Params(t) {
		if !c.p.R.TypeParam(p).IsLifetime {
			out = append(out, p)
		}
	}
	return out
}

// fnRetLifetime is info's declared return lifetime, or the sole lifetime
// its return type names deeper inside (`-> Array[View['a]]`).
func (c *borrowCheck) fnRetLifetime(info *sem.FnInfo) sem.EntityID {
	if info.RetLifetime != 0 {
		return info.RetLifetime
	}
	ret := c.p.R.Types.Node(info.Sig).Elem
	if c.p.R.Types.Kind(ret) == sem.KRef || c.p.R.Types.Kind(ret) == sem.KParam {
		return 0
	}
	return c.soleLifetime(ret)
}
