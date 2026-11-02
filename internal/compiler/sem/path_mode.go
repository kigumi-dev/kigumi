package sem

import (
	"strings"

	"kigumi/internal/compiler/syntax"
)

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

// pathMode walks from the leaf to the root and reads each link's declared type: a link whose base
// is a reference (a `&T` binding or field, a call returning `&T`) or a context key ends in pathRef.
func (c *checker) pathMode(n syntax.NodeID) pathMode {
	n = c.t.Unparen(n)
	switch c.t.Kind(n) {
	case syntax.MemberExpr:
		if ent := c.info.Uses[n]; ent != 0 && c.r.Entities[ent].Kind == EntContextKey {
			return pathRef
		}
		base := c.t.View(n).AsMember().Base().ID()
		if c.r.Types.Kind(c.vars.resolve(c.info.Types[base])) == KRef {
			return pathRef
		}
		return c.pathMode(base)
	case syntax.BracketExpr:
		return c.pathMode(c.t.View(n).AsBracket().Base().ID())
	case syntax.Ident:
		return c.bindingMode(c.info.Uses[n])
	}
	return pathTemp
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

// takeName names a taken place on one line: as written when that fits, else with each base that
// spans lines shown as `(...)`, as is a base that is no place or call (a join, a block).
func (c *checker) takeName(n syntax.NodeID) string {
	n = c.t.Unparen(n)
	switch c.t.Kind(n) {
	case syntax.Ident, syntax.MemberExpr, syntax.OptMemberExpr, syntax.BracketExpr, syntax.CallExpr, syntax.WsCallExpr:
	default:
		return "(...)"
	}
	if s := c.srcText(n); !strings.Contains(s, "\n") {
		return s
	}
	v := c.t.View(n)
	switch c.t.Kind(n) {
	case syntax.MemberExpr:
		return c.takeName(v.AsMember().Base().ID()) + "." + v.Tok().Text()
	case syntax.OptMemberExpr:
		return c.takeName(v.AsOptMember().Base().ID()) + "?." + v.Tok().Text()
	case syntax.BracketExpr:
		return c.takeName(v.AsBracket().Base().ID()) + "[..]"
	}
	return "(...)"
}

// borrowedLeaf: some value leaf of n is reached through a borrow, so a pattern on n binds by reference.
func (c *checker) borrowedLeaf(n syntax.NodeID) bool {
	for _, leaf := range c.valueLeaves(n) {
		if c.pathMode(leaf).borrowed() {
			return true
		}
	}
	return false
}
