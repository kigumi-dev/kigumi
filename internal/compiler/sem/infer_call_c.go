package sem

import (
	"math"

	"kigumi/internal/compiler/syntax"
)

// checkArgs has already checked arity and resolved named arguments, so args
// is at least len(params) long and already in parameter order.
func (c *checker) checkCArgs(n syntax.NodeID, name string, sn typeNode, args []syntax.NodeID) TypeID {
	params := sn.Args
	for i, a := range args {
		if i < len(params) {
			c.checkArg(a, params[i], 0, false, callResult{})
			continue
		}
		// A C variadic slot has no parameter type to coerce to. Do not unwrap
		// &T through readThrough: a borrow is not a C scalar or raw pointer, and
		// accepting it would pass the referent as a pointer-shaped value.
		t := c.vars.resolve(c.synthRead(a))
		if c.r.Types.Kind(t) == KUntyped {
			t = c.adopt(c.literalNode(a), c.variadicLiteralType(a, t))
		}
		if t != TyPoison && !c.r.cScalar(t) && c.r.Types.Kind(t) != KPtr {
			c.errAt(a, c.r.abiCode(t, cAbiType), c.r.TypeString(t))
		}
	}
	return c.vars.resolve(sn.Elem)
}

func (c *checker) variadicLiteralType(a syntax.NodeID, untyped TypeID) TypeID {
	if untyped != TyUntypedInt {
		return defaultOf(untyped)
	}
	if v, ok := c.info.Literals[c.literalNode(a)]; ok && v.Kind == constInt && v.Int.IsInt64() && v.Int.Int64() >= math.MinInt32 && v.Int.Int64() <= math.MaxInt32 {
		return TyI32
	}
	return TyI64
}
