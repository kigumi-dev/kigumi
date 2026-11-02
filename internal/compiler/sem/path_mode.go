package sem

import "kigumi/internal/compiler/syntax"

// pathMode is how a place expression is reached: from an owned binding or a temporary (whose
// other fields drop with it), through a lent binding, or through a reference.
type pathMode uint8

const (
	pathNone pathMode = iota
	pathOwned
	pathTemp
	pathLent
	pathRef
)

func (m pathMode) borrowed() bool { return m == pathLent || m == pathRef }

// pathMode walks from the leaf to the root and reads each link's declared type: a field or index through
// a reference or a context key ends in pathRef, a slice is a view of its source, a witness result a temporary.
func (c *checker) pathMode(n syntax.NodeID) pathMode {
	n = c.t.Unparen(n)
	switch c.t.Kind(n) {
	case syntax.MemberExpr:
		if ent := c.info.Uses[n]; ent != 0 && c.r.Entities[ent].Kind == EntContextKey {
			return pathRef
		}
		return c.linkMode(c.t.View(n).AsMember().Base().ID())
	case syntax.BracketExpr:
		switch c.bracketKind(n) {
		case bracketWitness, bracketValueSlice:
			return pathTemp
		case bracketSlice:
			return sliceMode(c.linkMode(c.t.View(n).AsBracket().Base().ID()))
		}
		return c.linkMode(c.t.View(n).AsBracket().Base().ID())
	case syntax.Ident:
		return c.bindingMode(c.info.Uses[n])
	}
	return pathTemp
}

// linkMode is how a projection out of base is reached. A binding holding a reference owns only the
// reference, so the binding reads as owned and what is reached through it as pathRef.
func (c *checker) linkMode(base syntax.NodeID) pathMode {
	if c.r.Types.Kind(c.vars.resolve(c.info.Types[base])) == KRef {
		return pathRef
	}
	return c.pathMode(base)
}

// pathDropOwner is the first type along n's links, leaf first, that defines a custom `drop`.
func (c *checker) pathDropOwner(n syntax.NodeID) TypeID {
	for n = c.t.Unparen(n); c.t.Kind(n) == syntax.MemberExpr; n = c.t.Unparen(c.t.View(n).AsMember().Base().ID()) {
		if bt := c.memberBaseType(n); c.customDrop(bt) && !c.dropSelf(c.t.View(n).AsMember().Base().ID()) {
			return bt
		}
	}
	return 0
}

func (c *checker) bindingMode(ent EntityID) pathMode {
	if ent == 0 {
		return pathNone
	}
	e := &c.r.Entities[ent]
	switch {
	case e.Kind == EntContextKey:
		return pathRef
	case e.Kind != EntLocal && e.Kind != EntParam:
		return pathNone
	case e.Flags&EfSelf != 0 && e.Flags&EfMove == 0:
		return pathLent
	}
	if c.lentCapture(ent) {
		return pathLent
	}
	return pathOwned
}

func (c *checker) lentCapture(ent EntityID) bool {
	if len(c.lambdas) == 0 {
		return false
	}
	ctx := c.lambdas[len(c.lambdas)-1]
	i, ok := ctx.captured[ent]
	return ok && ctx.list[i].Mode == capBorrow
}

// borrowedLeaf: some value leaf of n is reached through a borrow, so a pattern on n binds by reference.
// A leaf already refused binds as written, so its pattern adds no follow-on type error.
func (c *checker) borrowedLeaf(n syntax.NodeID) bool {
	for _, leaf := range c.valueLeaves(n) {
		if c.pathMode(leaf).borrowed() && !c.r.hasErrorAt(c.f, leaf) {
			return true
		}
	}
	return false
}
