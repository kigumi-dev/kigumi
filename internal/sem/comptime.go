package sem

import "kigumi/internal/syntax"

// ComptimeSite is a `comptime { ... }` block.
type ComptimeSite struct {
	File FileID
	Node syntax.NodeID
	Fn   EntityID
}

func (c *checker) synthComptimeBlock(n, inner syntax.NodeID) TypeID {
	saved := c.comptimeAt
	c.comptimeAt = len(c.lambdas) + 1
	t := c.synth(inner)
	c.comptimeAt = saved
	if !c.literalType(t) && !c.initRecordType(t, 0) {
		c.errAt(n, cComptimeResultType, c.r.TypeString(c.vars.settleLiterals(t)))
	}
	block := c.t.Span(inner)
	c.walkNodes(inner, func(id syntax.NodeID) {
		ent := c.info.Uses[id]
		if ent == 0 {
			return
		}
		e := &c.r.Entities[ent]
		if e.Kind != EntLocal && e.Kind != EntParam {
			return
		}
		if def := c.t.Span(e.Node); e.File != c.f || def.Start < block.Start || def.End > block.End {
			c.errAt(id, cComptimeRuntimeLocal, e.Name)
		}
	})
	c.r.Comptime = append(c.r.Comptime, ComptimeSite{File: c.f, Node: n, Fn: c.fn})
	return t
}

// leavesComptime reports whether a `?` or `await` at the current point would act on the comptime block itself.
func (c *checker) leavesComptime(n syntax.NodeID, what string) bool {
	if c.comptimeAt == 0 || len(c.lambdas) != c.comptimeAt-1 {
		return false
	}
	c.errAt(n, cComptimeFlow, what)
	return true
}

// literalType is what a comptime block may produce: what a literal can spell.
func (c *checker) literalType(t TypeID) bool {
	switch {
	case t == TyBool, t == TyChar, t == TyString, t == TyBytes, t == TyUntypedInt, t == TyUntypedFloat, t == TyPoison:
		return true
	}
	return c.r.Types.IsNumeric(t)
}

func (c *checker) walkNodes(n syntax.NodeID, f func(syntax.NodeID)) {
	f(n)
	for ch := range c.t.View(n).Children() {
		c.walkNodes(ch.ID(), f)
	}
}

// SetLiteral records the evaluated value of a comptime block.
func (r *Result) SetLiteral(f FileID, n syntax.NodeID, v Literal) {
	r.Files[f].Literals[n] = literalValue(v)
}

func literalValue(v Literal) constValue {
	cv := constValue{Int: v.Int, Float: v.Float, Str: v.Str, Bool: v.Bool, Char: v.Char}
	switch v.Kind {
	case LitInt:
		cv.Kind = constInt
	case LitFloat:
		cv.Kind = constFloat
	case LitString:
		cv.Kind = constString
	case LitBytes:
		cv.Kind = constBytes
	case LitBool:
		cv.Kind = constBool
	case LitChar:
		cv.Kind = constChar
	case LitRecord:
		cv.Kind = constRecord
		for _, f := range v.Fields {
			cv.Fields = append(cv.Fields, literalValue(f))
		}
	}
	return cv
}

// Only a const or static initializer may produce a record.
func (c *checker) initRecordType(t TypeID, depth int) bool {
	if c.fn == 0 || c.r.Fn(c.fn).InitOf == 0 || depth > 8 {
		return false
	}
	n := c.r.Types.Node(t)
	if n.Kind != KNamed || len(n.Args) > 0 {
		return false
	}
	info := c.r.typeDecl(n.Ent)
	if info.Form != FormRecord || len(info.Params) > 0 || len(info.Fields) == 0 {
		return false
	}
	for _, f := range info.Fields {
		ft := c.r.Entities[f].Type
		if !c.literalType(ft) && !c.initRecordType(ft, depth+1) {
			return false
		}
	}
	return true
}
