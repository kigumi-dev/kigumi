package sem

import "kigumi/internal/syntax"

// mutAliasArg is one call argument's scalar `&mut` place: the root local
// plus the field/index steps down to it, literal index values kept where seen.
type mutAliasArg struct {
	node  syntax.NodeID
	root  EntityID
	steps []mutAliasStep
}

type mutAliasStep struct {
	field EntityID
	index bool
	lit   bool
	val   int64
}

// checkMutAliasArgs rejects two `&mut` args that might name the same scalar
// storage: a true alias would silently keep only whichever boxed write lands last.
func (c *checker) checkMutAliasArgs(args []syntax.NodeID) {
	var seen []mutAliasArg
	for _, a := range args {
		arg := a
		if c.t.Kind(arg) == syntax.NamedArg {
			arg = syntax.NodeID(c.t.Nodes[arg].Lhs)
		}
		cand, ok := c.mutAliasArgOf(arg)
		if !ok {
			continue
		}
		for _, prev := range seen {
			if mutAliasConflict(prev, cand) {
				c.errAt(arg, cMutAliasArgs)
				break
			}
		}
		seen = append(seen, cand)
	}
}

// mutAliasArgOf reads the field/index chain of a scalar `&mut` argument, or
// reports ok false otherwise (a bare local already aliases its own cell).
func (c *checker) mutAliasArgOf(a syntax.NodeID) (mutAliasArg, bool) {
	if c.t.Kind(a) != syntax.BorrowExpr || c.t.Nodes[a].Lhs&syntax.FlagMut == 0 {
		return mutAliasArg{}, false
	}
	inner := syntax.NodeID(c.t.Nodes[a].Rhs)
	if _, ok := c.r.ScalarRefElem(c.vars.resolve(c.info.Types[a])); !ok {
		return mutAliasArg{}, false
	}
	root, steps, ok := c.mutAliasChain(inner)
	if !ok || len(steps) == 0 {
		return mutAliasArg{}, false
	}
	return mutAliasArg{node: a, root: root, steps: steps}, true
}

// mutAliasChain walks n down to its root local, recording each field/index
// step's own literal rather than collapsing it like sem/place.go's Place.
func (c *checker) mutAliasChain(n syntax.NodeID) (EntityID, []mutAliasStep, bool) {
	switch c.t.Kind(n) {
	case syntax.Ident:
		p, ok := c.buildPlace(n)
		if !ok {
			return 0, nil, false
		}
		return p.Root, nil, true
	case syntax.Paren:
		return c.mutAliasChain(syntax.NodeID(c.t.Nodes[n].Lhs))
	case syntax.MemberExpr:
		root, steps, ok := c.mutAliasChain(syntax.NodeID(c.t.Nodes[n].Lhs))
		if !ok {
			return 0, nil, false
		}
		fld := c.info.Uses[n]
		if fld == 0 || c.r.Entities[fld].Kind != EntField {
			return 0, nil, false
		}
		return root, append(steps, mutAliasStep{field: fld}), true
	case syntax.BracketExpr:
		root, steps, ok := c.mutAliasChain(syntax.NodeID(c.t.Nodes[n].Lhs))
		if !ok {
			return 0, nil, false
		}
		items := c.t.Children(syntax.NodeID(c.t.Nodes[n].Rhs))
		v, lit := c.info.Literals[items[0]]
		var iv int64
		if lit && v.Kind == constInt && v.Int.IsInt64() {
			iv = v.Int.Int64()
		} else {
			lit = false
		}
		return root, append(steps, mutAliasStep{index: true, lit: lit, val: iv}), true
	}
	return 0, nil, false
}

// mutAliasConflict: a differing root, length, field, or literal index pair
// rules out aliasing; an index neither side can resolve cannot.
func mutAliasConflict(a, b mutAliasArg) bool {
	if a.root != b.root || len(a.steps) != len(b.steps) {
		return false
	}
	for i := range a.steps {
		sa, sb := a.steps[i], b.steps[i]
		if sa.index != sb.index {
			return false
		}
		if !sa.index {
			if sa.field != sb.field {
				return false
			}
			continue
		}
		if sa.lit && sb.lit && sa.val != sb.val {
			return false
		}
	}
	return true
}
