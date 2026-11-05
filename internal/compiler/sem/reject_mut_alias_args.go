package sem

import "kigumi/internal/compiler/syntax"

// mutAliasArg is one call argument's scalar `&mut` place: the root local
// plus the field/index steps down to it, literal index values kept where seen.
type mutAliasArg struct {
	node   syntax.NodeID
	root   EntityID
	steps  []mutAliasStep
	scalar bool
}

type mutAliasStep struct {
	field EntityID
	index bool
	lit   bool
	val   int64
}

// checkMutAliasArgs rejects two `&mut` args that might name the same storage:
// a true alias would silently keep only whichever boxed write lands last.
func (c *checker) checkMutAliasArgs(args []syntax.NodeID) {
	var seen []mutAliasArg
	for _, a := range args {
		arg := a
		if c.t.Kind(arg) == syntax.NamedArg {
			arg = c.t.View(arg).AsNamedArg().Value().ID()
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

// mutAliasArgOf reads the place chain of any `&mut` argument, or reports ok
// false when the place cannot be resolved. A bare local is a chain with no steps.
func (c *checker) mutAliasArgOf(a syntax.NodeID) (mutAliasArg, bool) {
	return c.borrowAliasArgOf(a, true)
}

// borrowAliasArgOf reads a borrow's place chain; requireMut selects the `&mut`
// form. A shared borrow still conflicts with a mutable receiver.
func (c *checker) borrowAliasArgOf(a syntax.NodeID, requireMut bool) (mutAliasArg, bool) {
	if c.t.Kind(a) != syntax.BorrowExpr {
		return mutAliasArg{}, false
	}
	if requireMut && c.t.View(a).AsBorrow().Flags()&syntax.FlagMut == 0 {
		return mutAliasArg{}, false
	}
	inner := c.t.View(a).AsBorrow().Operand().ID()
	_, scalar := c.r.ScalarRefElem(c.vars.resolve(c.info.Types[a]))
	root, steps, ok := c.mutAliasChain(inner)
	if !ok {
		return mutAliasArg{}, false
	}
	return mutAliasArg{node: a, root: root, steps: steps, scalar: scalar}, true
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
		return c.mutAliasChain(c.t.View(n).AsParen().Inner().ID())
	case syntax.MemberExpr:
		root, steps, ok := c.mutAliasChain(c.t.View(n).AsMember().Base().ID())
		if !ok {
			return 0, nil, false
		}
		fld := c.info.Uses[n]
		if fld == 0 || c.r.Entities[fld].Kind != EntField {
			return 0, nil, false
		}
		return root, append(steps, mutAliasStep{field: fld}), true
	case syntax.BracketExpr:
		root, steps, ok := c.mutAliasChain(c.t.View(n).AsBracket().Base().ID())
		if !ok {
			return 0, nil, false
		}
		v, lit := c.info.Literals[c.t.View(n).AsBracket().Args().ItemID(0)]
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

// mutAliasConflict reports whether two places may overlap: same root and one
// chain is a prefix of the other (an aggregate and a field below it), or both
// reach the same step with compatible fields and indices.
func mutAliasConflict(a, b mutAliasArg) bool {
	if a.root != b.root {
		return false
	}
	n := len(a.steps)
	if len(b.steps) < n {
		n = len(b.steps)
	}
	for i := 0; i < n; i++ {
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
	// Equal prefixes overlap. Two non-scalar borrows of the same place are
	// already rejected by the exclusivity check; reporting here too would
	// duplicate the diagnostic, so only pairs with a scalar side use this pass.
	return a.scalar || b.scalar
}
