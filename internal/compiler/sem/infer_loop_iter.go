package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) iterElement(loop, n syntax.NodeID, iter TypeID) TypeID {
	tt := c.r.Types
	if iter == TyPoison {
		return TyPoison
	}
	if tt.Kind(iter) == KUntyped {
		iter = c.adopt(c.literalNode(n), defaultOf(iter))
	}
	// Element keeps the head's borrow, as &T/&mut T bind an argument.
	if tt.Kind(iter) == KRef {
		if inner := c.vars.resolve(tt.Node(iter).Elem); c.isRangeType(n, inner) {
			c.errAt(n, cNotIterable, iter)
			return TyPoison
		}
		mut := tt.Node(iter).Flags&flagMut != 0
		elem := c.iterElement(loop, n, c.vars.resolve(tt.Node(iter).Elem))
		if elem == TyPoison {
			return TyPoison
		}
		return tt.Ref(elem, mut)
	}
	if iter == TyString {
		c.errAt(n, cForString)
		return TyPoison
	}
	if iter == TyBytes {
		c.info.Calls[loop] = CallInfo{Kind: CallIter}
		return TyU8
	}
	node := tt.Node(iter)
	if node.Kind == KNamed && (node.Ent == tt.arrayEnt || c.r.isFixedArray(iter)) {
		c.info.Calls[loop] = CallInfo{Kind: CallIter}
		return node.Args[0]
	}
	if c.isRangeType(n, iter) {
		if arg := c.vars.resolve(node.Args[0]); !tt.IsInteger(arg) && !c.vars.pendingLiteral(arg) && tt.Kind(arg) != KUntyped {
			c.errAt(n, cNotIterable, iter)
			return TyPoison
		}
		c.info.Calls[loop] = CallInfo{Kind: CallIter}
		return node.Args[0]
	}
	if me := c.r.mapEntity(); node.Kind == KNamed && me != 0 && node.Ent == me {
		tup, _ := tt.tupleEntity(2)
		c.info.Calls[loop] = CallInfo{Kind: CallIterMap, Callee: me}
		return tt.Named(tup, []TypeID{node.Args[0], node.Args[1]})
	}
	c.errAt(n, cNotIterable, iter)
	return TyPoison
}
