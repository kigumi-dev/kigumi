package sem

import (
	"strings"

	"kigumi/internal/syntax"
)

// IsTruncate reports whether fn is an integer type's prelude `truncate`.
func (r *Result) IsTruncate(fn EntityID) bool {
	e := &r.Entities[fn]
	if e.Kind != EntFn || e.Name != "truncate" || e.Flags&EfStd == 0 {
		return false
	}
	owner := r.Fn(fn).Owner
	return owner != 0 && r.Types.IsInteger(r.Entities[owner].Type)
}

// IsConversion reports whether fn is a numeric `toX` conversion or `truncate`.
func (r *Result) IsConversion(fn EntityID) bool {
	return strings.HasPrefix(r.Entities[fn].Name, "to") && r.Fn(fn).Owner != 0 || r.IsTruncate(fn)
}

// truncateCheck requires a concrete integer target no wider than the receiver.
func (c *checker) truncateCheck(n syntax.NodeID, fn EntityID, typeArgs, inst []TypeID) {
	if !c.r.IsTruncate(fn) || len(inst) != 1 {
		return
	}
	tt := c.r.Types
	if len(typeArgs) != 1 {
		if len(typeArgs) == 0 {
			c.errAt(n, cTruncateNoTarget)
			c.vars.poison(inst[0])
		}
		return
	}
	target := c.vars.resolve(inst[0])
	self := c.r.Entities[c.r.Fn(fn).Owner].Type
	if target != TyPoison && (!tt.IsInteger(target) || tt.Width(target) > tt.Width(self)) {
		recv := c.r.TypeString(self)
		c.errAt(n, cTruncateTarget, c.r.TypeString(target), recv, recv)
	}
}
