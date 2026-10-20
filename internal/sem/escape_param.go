package sem

import (
	"strings"

	"kigumi/internal/syntax"
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

func (r *Result) escapingMarks(types []TypeID, escaping []bool) string {
	set := make([]byte, (len(types)+7)/8)
	for i, t := range types {
		if escaping[i] && r.Types.Kind(t) == KParam {
			set[i/8] |= 1 << (i % 8)
		}
	}
	return strings.TrimRight(string(set), "\x00")
}

// Keeps a written `escaping` on a bounded type parameter, which the type itself does not carry.
func (r *Result) ParamTypeString(p EntityID) string {
	e := &r.Entities[p]
	if e.Flags&EfEscaping != 0 && r.Types.Kind(e.Type) == KParam {
		return "escaping " + r.TypeString(e.Type)
	}
	return r.TypeString(e.Type)
}

func (r *Result) escapingForm(p EntityID) string {
	if t := r.Entities[p].Type; r.Types.Kind(t) == KParam {
		return "`escaping " + r.TypeString(t) + "`"
	}
	return "`escaping fn(...) -> ...`"
}
