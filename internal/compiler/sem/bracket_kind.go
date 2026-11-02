package sem

import "kigumi/internal/compiler/syntax"

// bracketKind is what a resolved `[]` reads: an element in place, a slice sharing the source's element objects,
// a slice of plain values (copies), a substring (new string), or a user `index` witness's fresh result.
type bracketKind uint8

const (
	bracketOther bracketKind = iota
	bracketElement
	bracketSlice
	bracketValueSlice
	bracketSubstring
	bracketWitness
)

func (c *checker) bracketKind(n syntax.NodeID) bracketKind {
	switch c.info.Calls[n].Kind {
	case CallIndexWitness:
		return bracketWitness
	case CallIndex:
	default:
		return bracketOther
	}
	b := c.t.View(n).AsBracket()
	if b.Args().Len() != 1 {
		return bracketOther
	}
	tt := c.r.Types
	recv := c.vars.resolve(c.info.Types[b.Base().ID()])
	if tt.Kind(recv) == KRef {
		recv = c.vars.resolve(tt.Node(recv).Elem)
	}
	idx := c.vars.resolve(c.info.Types[b.Args().ItemID(0)])
	array := tt.Kind(recv) == KNamed && tt.Node(recv).Ent == tt.arrayEnt || c.r.isFixedArray(recv)
	rng := tt.Kind(idx) == KNamed && tt.Node(idx).Ent != 0 && tt.Node(idx).Ent == c.r.langItems["Range"]
	switch {
	case array && idx == TyUsize:
		return bracketElement
	case array && rng && c.plainValue(tt.Node(recv).Args[0]):
		return bracketValueSlice
	case array && rng:
		return bracketSlice
	case rng && (recv == TyString || recv == TyBytes):
		return bracketSubstring
	}
	return bracketOther
}

// freshBracket: the bracket's value is built anew, so a borrow of it ends with its statement.
func (c *checker) freshBracket(n syntax.NodeID) bool {
	k := c.bracketKind(n)
	return k == bracketSlice || k == bracketValueSlice || k == bracketSubstring || k == bracketWitness
}

// plainValue: a Copy element without a `mut` field, which a slice copies with no shared object to write.
func (c *checker) plainValue(elem TypeID) bool {
	return c.copyNow(elem) && !c.r.holdsMutField(c.vars.resolve(elem), map[TypeID]bool{})
}

// indexRoot is the bracket n bottoms out at through parens and field projections.
func (c *checker) indexRoot(n syntax.NodeID) (syntax.NodeID, bool) {
	for {
		switch c.t.Kind(n) {
		case syntax.Paren, syntax.MemberExpr, syntax.OptMemberExpr:
			n = c.t.View(n).Base().ID()
		case syntax.BracketExpr:
			return n, true
		default:
			return 0, false
		}
	}
}

// sliceMode: a slice is a view of its source, so it is borrowed unless the source is a temporary.
func sliceMode(source pathMode) pathMode {
	if source == pathTemp || source.borrowed() {
		return source
	}
	return pathRef
}
