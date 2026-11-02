package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// Differing signatures for the same name are an ambiguity error, not a
// silent first pick; dedup keys on interface instantiation, not the requirement entity.
func (c *checker) ifaceMember(n syntax.NodeID, t TypeID, name string) (req EntityID, sig, iface TypeID, ambiguous bool) {
	tt := c.r.Types
	node := tt.Node(t)
	var ifaces []TypeID
	switch node.Kind {
	case KParam:
		for _, con := range c.r.typeParam(node.Ent).Constraints {
			if con.Kind == CIface {
				ifaces = append(ifaces, con.Type)
			}
		}
	case KIface:
		ifaces = append(ifaces, t)
	}
	type found struct {
		req   EntityID
		sig   TypeID
		iface TypeID
	}
	var founds []found
	for _, it := range ifaces {
		in := tt.Node(it)
		info := c.r.iface(in.Ent)
		req := c.r.requirement(in.Ent, name)
		if req == 0 {
			continue
		}
		if slices.ContainsFunc(founds, func(f found) bool { return f.iface == it }) {
			continue
		}
		subst := map[EntityID]TypeID{info.SelfParam: t}
		for i, p := range info.Params {
			if i < len(in.Args) {
				subst[p] = in.Args[i]
			}
		}
		founds = append(founds, found{req, tt.Subst(c.r.Fn(req).Sig, subst), it})
	}
	switch len(founds) {
	case 0:
		return 0, 0, 0, false
	case 1:
		return founds[0].req, founds[0].sig, founds[0].iface, false
	}
	for _, f := range founds[1:] {
		if !c.r.sameRequirementSig(founds[0].req, founds[0].sig, f.req, f.sig) {
			c.errAt(n, cAmbiguousMember, name, founds[0].iface, f.iface)
			return 0, TyPoison, 0, true
		}
	}
	return founds[0].req, founds[0].sig, founds[0].iface, false
}

func (c *checker) callRequirement(n, callee, base syntax.NodeID, recv TypeID, req EntityID, sig, iface TypeID, typeArgs []TypeID, args []syntax.NodeID, want TypeID) TypeID {
	c.info.Uses[callee] = req
	c.checkReceiver(base, recv, req)
	c.noteWitnessRecv(base, recv, req, iface)
	c.noteSelfRecvCall(base, req, true, iface)
	var paramEnt EntityID
	if valueType := c.r.Types.Node(recv); c.r.Types.Kind(recv) == KParam || c.r.Types.Kind(recv) == KRef && c.r.Types.Kind(valueType.Elem) == KParam {
		param := recv
		if c.r.Types.Kind(recv) == KRef {
			param = valueType.Elem
		}
		paramEnt = c.r.Types.Node(param).Ent
	}
	// An existential receiver has no witness to defer to, so the call stays impure regardless of `pure?`.
	if paramEnt != 0 && c.r.Fn(req).EffectPoly {
		c.deferToWitnessReq(paramEnt, req)
		c.edge(EffectEdge{Kind: EdgeWitnessCall, Target: req, Node: n})
	} else {
		c.edge(EffectEdge{Kind: EdgeCall, Target: req, Param: c.ifaceRecvEnt(base, recv), Node: n})
		c.noteIfaceRecvFn(n, base)
	}
	declared := sig
	sig, subst := c.instantiateRequirement(n, req, sig, typeArgs)
	from := len(*c.edges)
	ret, plan := c.checkArgs(n, c.r.Entities[req].Name, sig, args, nil, want, c.r.Fn(req).Params, false, c.r.Fn(req).Declared&EffAsync != 0)
	c.taintRecvResult(n, ret, base)
	c.requirementResult(n, ret, from, args, paramEnt != 0, c.r.Fn(req).Declared&EffAsync != 0)
	effArgs := args
	if plan != nil {
		effArgs = plan.placed()
	}
	c.argContracts(effArgs, declared, subst, 0, 0)
	if c.r.Fn(req).Recv == RecvMut {
		c.checkReentrantCapture(base, effArgs)
		c.recordReqFieldCallSite(n, req, base, effArgs)
	}
	c.checkSelfAliasArgs(base, c.r.Fn(req).Recv, c.r.Fn(req).Declared&EffAsync != 0, effArgs)
	call := CallInfo{Kind: CallMethod, Callee: req, Recv: c.r.Fn(req).Recv, Variadic: c.variadicForm(effArgs, sig), Plan: plan}
	if paramEnt != 0 {
		call.Witness = c.witnessLocal(n, paramEnt, req)
	}
	c.info.Calls[n] = call
	c.noteRecvStores(n, base, req, effArgs)
	c.rejectLentRecv(base, effArgs)
	c.touched = append(c.touched, n)
	return ret
}

func (c *checker) isMethodValue(n syntax.NodeID) bool {
	call, ok := c.info.Calls[c.literalNode(n)]
	return ok && call.Kind == CallMethodValue
}

// Resolves to the width that uniquely declares name, else the untyped default
// always picks a concrete type so a missing member names it like a bound variable would.
func (c *checker) resolveLiteralReceiver(untyped TypeID, name string) (TypeID, bool) {
	var pool []TypeID
	switch untyped {
	case TyUntypedInt:
		for t := TyI8; t <= TyIsize; t++ {
			pool = append(pool, t)
		}
	case TyUntypedFloat:
		pool = []TypeID{TyF32, TyF64}
	default:
		return 0, false
	}
	var only TypeID
	for _, t := range pool {
		if c.memberSetHere(t, name) != 0 {
			if only != 0 {
				only = 0
				break
			}
			only = t
		}
	}
	if only != 0 {
		return only, true
	}
	return defaultOf(untyped), true
}

func (c *checker) overloadValue(n syntax.NodeID, want TypeID) (TypeID, bool) {
	head := c.resolveHead(n)
	if head.kind != headSet {
		return 0, false
	}
	var match []EntityID
	for _, m := range c.r.Overloads[head.set].Members {
		if c.sameFnShape(c.r.Fn(m).Sig, want) {
			match = append(match, m)
		}
	}
	if len(match) != 1 {
		c.errAt(n, cOverloadValueAmbig, c.r.Overloads[head.set].Name)
		return TyPoison, true
	}
	c.r.useEntity(c.f, n, match[0])
	c.info.Uses[n] = match[0]
	c.recordCtxValueRef(n, match[0])
	return c.r.Fn(match[0]).Sig, true
}
