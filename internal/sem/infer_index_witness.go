package sem

import "kigumi/internal/syntax"

// indexWitnessType resolves `recv[idx]` against recv's own `index` member, fixing an
// untyped index literal to its parameter type here, before any builtin usize coercion runs.
func (c *checker) indexWitnessType(n, idx syntax.NodeID, recv, index TypeID) TypeID {
	tt := c.r.Types
	set := c.memberSetHere(recv, "index")
	if set == 0 {
		c.errAt(n, cNotIndexable, recv)
		return TyPoison
	}
	m := c.visibleMembers(set)[0]
	sig := tt.Node(c.r.memberSig(recv, m))
	want := TyPoison
	if len(sig.Args) == 1 {
		want = sig.Args[0]
	}
	switch {
	case tt.Kind(index) == KUntyped:
		index = c.adopt(c.literalNode(idx), want)
	case want != TyPoison:
		if _, ok := c.vars.index(index); ok {
			c.vars.unify(index, want)
		}
		index = c.vars.resolve(index)
	default:
		index = c.vars.resolve(index)
	}
	if len(sig.Args) == 1 && sig.Args[0] == index {
		c.info.Calls[n] = CallInfo{Kind: CallIndexWitness, Callee: m, Plan: identityPlan([]syntax.NodeID{idx})}
		c.edge(EffectEdge{Kind: EdgeCall, Target: m, Node: n})
		c.deferWitnesses(n, m, recv, nil)
		return c.indexValueResult(n, sig.Elem)
	}
	c.errAtHelp(idx, usizeHelp(TyUsize, index), cIndexType, index)
	return TyPoison
}

func (c *checker) isMethodRef(base syntax.NodeID) bool {
	if c.t.Kind(base) != syntax.MemberExpr {
		return false
	}
	tt := c.r.Types
	member := c.t.View(base).AsMember()
	recv := c.vars.resolve(c.info.Types[member.Base().ID()])
	if tt.Kind(recv) == KRef {
		recv = tt.Node(recv).Elem
	}
	name := member.Member().Text()
	isField := tt.Kind(recv) == KNamed && c.r.findField(tt.Node(recv).Ent, name) != 0
	return !isField && c.memberSetHere(recv, name) != 0
}

// A method named without a call takes its type arguments here, fixing the instantiation its method value made.
func (c *checker) methodRefTypeArgs(n, base syntax.NodeID, items syntax.ListView, ref TypeID) TypeID {
	m := c.info.Uses[base]
	var params []EntityID
	if m != 0 && c.r.Entities[m].Kind == EntFn {
		params = c.ownGenerics(m)
	}
	var args []TypeID
	for i, a := range items.EnumerateIDs() {
		args = append(args, c.r.resolveExprGenericArg(c.f, c.scope, a, entAt(params, i)))
	}
	if ref == TyPoison || m == 0 {
		return ref
	}
	inst := c.methodValueInst(m, base)
	c.nameTruncateValue(base)
	if len(args) != len(inst) {
		c.errAt(n, cTypeArgCount, c.r.Entities[m].Name, len(inst), plural(len(inst)), len(args))
		for _, t := range inst {
			c.vars.poison(t)
		}
		return TyPoison
	}
	for i, t := range args {
		if t == TyPoison {
			c.vars.poison(inst[i])
			continue
		}
		c.vars.unify(inst[i], t)
	}
	return c.vars.resolve(ref)
}

func (c *checker) methodValueInst(m EntityID, base syntax.NodeID) []TypeID {
	insts := c.r.Instances[m]
	for i := len(insts) - 1; i >= 0; i-- {
		if insts[i].File == c.f && insts[i].Node == base {
			return insts[i].Args
		}
	}
	return nil
}
