package sem

import (
	"strings"

	"kigumi/internal/compiler/syntax"
)

// An aliased fn type takes the escaping bit; a bounded type parameter is marked by the caller instead.
func (r *Result) escapingParamType(f FileID, node syntax.NodeID, typ TypeID) (TypeID, bool) {
	switch r.Types.Kind(typ) {
	case KPoison:
		return typ, false
	case KFn:
		if r.Types.IsCFn(typ) {
			return typ, true
		}
		return r.Types.WithEscaping(typ), true
	case KParam:
		if r.fnShapeOf(typ) != 0 {
			return typ, true
		}
		r.errAt(f, node, cEscapingUnboundedParam, r.Entities[r.Types.Node(typ).Ent].Name)
		return typ, false
	}
	r.errAt(f, node, cEscapingNotCallable, r.TypeString(typ))
	return typ, false
}

// A granted `escaping` is marked on a type parameter or, for a variadic parameter, on its array.
func (r *Result) escapingMarks(types []TypeID, escaping []bool) string {
	set := make([]byte, (len(types)+7)/8)
	for i, t := range types {
		if k := r.Types.Kind(t); escaping[i] && (k == KParam || k == KNamed) {
			set[i/8] |= 1 << (i % 8)
		}
	}
	return strings.TrimRight(string(set), "\x00")
}

// Keeps a written `escaping` on a bounded type parameter, which the type itself does not carry.
func (r *Result) ParamTypeString(p EntityID) string {
	e := &r.Entities[p]
	if e.Flags&EfEscaping != 0 && e.Flags&EfVariadic != 0 {
		return "...escaping " + r.TypeString(r.callbackType(e))
	}
	if e.Flags&EfEscaping != 0 && r.Types.Kind(e.Type) == KParam {
		return "escaping " + r.TypeString(e.Type)
	}
	return r.TypeString(e.Type)
}

func (c *checker) sameButEscaping(a, b TypeID) bool {
	mark := c.vars.mark()
	ok := c.vars.unifyIgnoringEscaping(a, b)
	c.vars.rollback(mark)
	return ok
}

func (c *checker) typeText(t TypeID) string {
	c.r.varNames = c.vars.names
	defer func() { c.r.varNames = nil }()
	return c.r.TypeString(c.vars.resolve(t))
}

func (r *Result) escapingForm(p EntityID) string {
	e := &r.Entities[p]
	dots := ""
	if e.Flags&EfVariadic != 0 {
		dots = "..."
	}
	if t := r.callbackType(e); r.Types.Kind(t) == KParam {
		return "`" + dots + "escaping " + r.TypeString(t) + "`"
	}
	return "`" + dots + "escaping fn(...) -> ...`"
}
