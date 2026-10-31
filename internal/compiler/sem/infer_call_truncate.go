package sem

import (
	"strings"

	"kigumi/internal/compiler/syntax"
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
	if len(typeArgs) != 1 {
		if len(typeArgs) == 0 {
			c.errAt(n, cTruncateNoTarget)
			c.vars.poison(inst[0])
		}
		return
	}
	c.truncateTarget(n, fn, c.vars.resolve(inst[0]))
}

func (c *checker) truncateTarget(n syntax.NodeID, fn EntityID, target TypeID) {
	tt := c.r.Types
	self := c.r.Entities[c.r.Fn(fn).Owner].Type
	if target != TyPoison && (!tt.IsInteger(target) || tt.Width(target) > tt.Width(self)) {
		recv := c.r.TypeString(self)
		c.errAt(n, cTruncateTarget, c.r.TypeString(target), recv, recv)
	}
}

type truncateValue struct {
	node  syntax.NodeID
	fn    EntityID
	inst  []TypeID
	named bool
}

// A method value of `truncate` is checked once inference settles; only an explicit `[T]` names the target.
func (c *checker) deferTruncateValue(n syntax.NodeID, fn EntityID, inst []TypeID) {
	if c.r.IsTruncate(fn) && len(inst) == 1 {
		c.methodValues.truncates = append(c.methodValues.truncates, truncateValue{node: n, fn: fn, inst: inst})
	}
}

func (c *checker) nameTruncateValue(n syntax.NodeID) {
	for i := range c.methodValues.truncates {
		if c.methodValues.truncates[i].node == n {
			c.methodValues.truncates[i].named = true
		}
	}
}

func (c *checker) runTruncateValueChecks() {
	pending := c.methodValues.truncates
	c.methodValues.truncates = nil
	for _, v := range pending {
		if !v.named {
			c.errAt(v.node, cTruncateNoTarget)
			c.vars.poison(v.inst[0])
			continue
		}
		c.truncateTarget(v.node, v.fn, c.vars.resolve(v.inst[0]))
	}
}
