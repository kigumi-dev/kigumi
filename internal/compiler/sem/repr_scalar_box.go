package sem

import "kigumi/internal/compiler/syntax"

// ScalarBoxes reports whether a t crossing into a boxed slot may take a heap box on native.
func (r *Result) ScalarBoxes(t TypeID) bool {
	// Every i8/u8 value fits in the native integer cache, and Bool boxes are immortal.
	if t == TyBool || t == TyI8 || t == TyU8 {
		return false
	}
	return r.RegScalar(t) || WideScalar(t) || r.Types.Kind(t) == KPtr || r.Types.IsCFn(t)
}

// boxCharged reports whether a box edge over t allocates; a witness boxes a scalar only in a shared body, which the built program decides.
func (r *Result) boxCharged(why uint8, t TypeID) bool {
	if witnessWhy(why) {
		return r.BoxAllocates(t)
	}
	return r.BoxAllocates(t) || r.ScalarBoxes(t)
}

const (
	// whyScalarCross marks a scalar argument that a call through a function value receives in a box.
	whyScalarCross = 20
	// whyScalarStore marks a scalar stored into a slot that holds it boxed.
	whyScalarStore = 21
	// whyClosureScalar marks a scalar a closure takes or returns in a box.
	whyClosureScalar = 22
)

var scalarCodes = map[uint8]Code{whyScalarCross: cAllocScalarArg, whyScalarStore: cAllocScalarStore, whyClosureScalar: cAllocClosureScalar, whyBoxedArg: cAllocBoxedArg}

// A constant argument is handed over as its immortal, so it needs no box.
func (c *checker) crossScalarArgs(args []syntax.NodeID, why uint8) {
	for _, a := range args {
		if c.constScalar(a) {
			continue
		}
		if p := c.paramRead(a); p != 0 {
			c.noteBoxedParam(p)
			continue
		}
		c.scalarEdge(a, c.argType(a), why)
	}
}

// scalarStore charges the box a store into lhs takes; ref names the reference binding written through, if any.
func (c *checker) scalarStore(n, lhs syntax.NodeID, t TypeID, ref EntityID) {
	if elem, ok := c.r.MutRefElem(c.vars.resolve(t)); ok {
		t = elem
	}
	if n != lhs && c.sharesBox(lhs, n) {
		return
	}
	if p := c.paramRead(n); n != lhs && p != 0 {
		c.noteBoxedParam(p)
		return
	}
	c.scalarEdgeAt(n, t, whyScalarStore, ref)
}

// A read from a container slot, or from the place being written, hands over a box that already exists.
func (c *checker) sharesBox(lhs, rhs syntax.NodeID) bool {
	rhs = c.t.Unparen(rhs)
	if k := c.t.Kind(rhs); k != syntax.MemberExpr && k != syntax.BracketExpr {
		return false
	}
	if root := c.rootOf(rhs); root != 0 && root == c.rootOf(lhs) {
		return true
	}
	return c.inContainerSlot(rhs)
}

// A field or element of a direct aggregate is a register read, even through a reference.
func (c *checker) inContainerSlot(n syntax.NodeID) bool {
	bt := c.vars.resolve(c.info.Types[c.t.View(n).Base().ID()])
	if c.r.Types.Kind(bt) == KRef {
		bt = c.r.Types.Node(bt).Elem
	}
	return !c.r.DirectAgg(bt)
}

func (c *checker) scalarEdge(n syntax.NodeID, t TypeID, why uint8) { c.scalarEdgeAt(n, t, why, 0) }

func (c *checker) scalarEdgeAt(n syntax.NodeID, t TypeID, why uint8, target EntityID) {
	if t = c.vars.resolve(t); !c.r.Types.ContainsVar(t) && c.r.ScalarBoxes(t) {
		c.edge(EffectEdge{Kind: EdgeAlloc, Node: n, Type: t, Why: why, Target: target, Captured: c.capturedRead(n)})
	}
}

func (c *checker) constScalar(n syntax.NodeID) bool {
	n = c.t.Unparen(n)
	if _, ok := c.info.Literals[n]; ok || c.t.Kind(n) == syntax.CharLit {
		return true
	}
	ent := c.info.Uses[n]
	return ent != 0 && c.r.Entities[ent].Kind == EntConst
}

func (c *checker) storeRef(lhs syntax.NodeID, ref bool) EntityID {
	if !ref || !c.isIdentPlace(lhs) {
		return 0
	}
	return c.info.Uses[lhs]
}

// A closure handing back what a reference or a container slot holds returns the box that is already there.
func (c *checker) readsBox(n syntax.NodeID) bool {
	n = c.t.Unparen(n)
	switch c.t.Kind(n) {
	case syntax.Ident:
		return c.r.Types.Kind(c.vars.resolve(c.info.Types[n])) == KRef || c.paramRead(n) != 0
	case syntax.MemberExpr, syntax.BracketExpr:
		return c.inContainerSlot(n)
	}
	return false
}

// A closure reads a capture from its environment, which holds the value boxed already.
func (c *checker) capturedRead(n syntax.NodeID) bool {
	n = c.t.Unparen(n)
	if len(c.lambdas) == 0 || c.t.Kind(n) != syntax.Ident {
		return false
	}
	_, ok := c.lambdas[len(c.lambdas)-1].captured[c.info.Uses[n]]
	return ok
}
