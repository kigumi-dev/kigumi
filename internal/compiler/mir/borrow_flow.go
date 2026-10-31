package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type callSlot struct {
	arg    midir.LocalID
	params []sem.EntityID
	target bool
	// holder marks an argument handed over by value whose type holds a `&mut`: the callee may store through it.
	holder   bool
	contents bool
	exact    sem.TypeID
}

func (c *borrowCheck) callSlots(info *sem.FnInfo, args []midir.LocalID, recvIdx int) []callSlot {
	var out []callSlot
	start := recvIdx
	if info.Recv != sem.RecvNone {
		start++
		if recvIdx < len(args) {
			out = append(out, callSlot{arg: args[recvIdx], params: c.recvLifetimeParams(info), target: info.Recv == sem.RecvMut,
				holder: info.Recv == sem.RecvMove && c.holdsMutPlace(args[recvIdx]), contents: info.Recv != sem.RecvMove})
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
		out = append(out, callSlot{arg: args[start+i], params: params, target: ref && c.p.R.Types.RefMut(decl),
			holder: !ref && (c.holdsMutPlace(args[start+i]) || c.capturesHolder(args[start+i])), contents: ref && c.p.R.Local(prm).Lifetime == 0, exact: decl})
	}
	return out
}

func sharesParam(a, b []sem.EntityID) bool {
	return slices.ContainsFunc(a, func(p sem.EntityID) bool { return slices.Contains(b, p) })
}

// callFlows lists, for each `&mut` / `mut self` argument or by-value `&mut` holder whose place names a lifetime
// or a type parameter, the other arguments whose declared types share a parameter with it.
func (c *borrowCheck) callFlows(in midir.Inst) []storeFlow {
	e, args := c.flowCallee(in)
	if e == 0 {
		return nil
	}
	info := c.p.R.Fn(e)
	if info.Recv != sem.RecvMut && !slices.ContainsFunc(info.Params, func(p sem.EntityID) bool {
		return c.p.R.Types.RefMut(c.p.R.Entity(p).Type)
	}) && !slices.ContainsFunc(args, func(a midir.LocalID) bool { return c.holdsMutPlace(a) || c.capturesHolder(a) }) {
		return nil
	}
	slots := c.callSlots(info, args, 0)
	c.linkSlots(info, slots)
	var out []storeFlow
	for i, t := range slots {
		if !t.target && !t.holder {
			continue
		}
		kind := c.targetKindOf(t.arg)
		if t.holder && c.closureLiteral(t.arg) != nil {
			kind = targetLifetime
		}
		if kind == targetNone {
			continue
		}
		fl := storeFlow{target: t.arg, ref: t.target, byValue: t.holder, generic: kind == targetParam, callee: e, param: i, args: append(slotArgs(slots), args[min(len(slots), len(args)):]...)}
		params := c.linkedParams(slots, i)
		for j, v := range slots {
			if i != j && kind != targetDeferred && sharesParam(params, v.params) {
				fl.values = append(fl.values, flowArg{arg: v.arg, contents: v.contents, param: j})
			}
		}
		if lit := c.closureLiteral(t.arg); t.holder && lit != nil {
			fl.values = append(c.closureFed(c.p.ByEnt[e], i, lit, fl.values), c.captureValues(lit)...)
		}
		if at := c.selfSources(e, i, slots); at != nil {
			fl.values = append(fl.values, flowArg{arg: t.arg, contents: t.contents, param: i, at: at})
		}
		fl.values = append(fl.values, c.tieValues(info, slots, t, fl.values)...)
		if fl.values != nil {
			out = append(out, fl)
		}
	}
	return out
}

func (c *borrowCheck) placeOf(l midir.LocalID) sem.TypeID {
	if _, place, _, ok := c.storeRoot(storeFlow{target: l, ref: true}); ok {
		return place
	}
	return 0
}

// genericFlowSource ties a generic call's result to every argument whose declared type names a type parameter the result
// names, widened through callbacks returning one; a result named only by a type parameter keeps just this frame's borrows.
func (c *borrowCheck) genericFlowSource(in midir.Inst, recvIdx int) (*borrowInfo, bool) {
	info := c.p.R.Fn(in.Ent)
	owned := !c.resultMayCarryLifetime(c.f.Locals[in.Dst].Type)
	if c.fnRetLifetime(info) != 0 || owned && !c.namesTypeParam(c.f.Locals[in.Dst].Type) {
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
		b, ok := c.flowInfo(flowArg{arg: s.arg, contents: s.contents})
		if ok && owned {
			b = c.frameBorrows(b)
			ok = b != nil
		}
		if ok {
			srcs = append(srcs, b)
			extra = extra || !bare || s.exact != ret
		}
	}
	if bare && !extra || owned && len(srcs) == 0 {
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
