package sem

import (
	"strconv"
	"strings"
)

type cSymbolSite struct {
	ent    EntityID
	export bool
	shape  string
}

func (r *Result) checkCSymbols() {
	first := map[string]cSymbolSite{}
	for id := range r.Entities {
		e := &r.Entities[id]
		if e.Kind != EntFn || e.File == 0 {
			continue
		}
		info := r.Fn(EntityID(id))
		if info.Abi == "" && info.Export == "" || info.Sig == 0 {
			continue
		}
		name := r.FnLinkNameOf(EntityID(id))
		here := cSymbolSite{EntityID(id), info.Export != "", r.cSignatureShape(info.Sig)}
		if here.export && info.Export != "interrupt" {
			if libcSymbols[name] && !(r.bareTarget() && compilerCalledSymbols[name]) {
				r.errAt(e.File, e.Node, cCSymbolConflict, name, "is already defined by the C library")
				continue
			}
			if strings.HasPrefix(name, "rt_") && !strings.HasPrefix(name, "rt_sys_") {
				r.errAt(e.File, e.Node, cCSymbolConflict, name, "is reserved for the Kigumi runtime")
				continue
			}
		}
		prev, seen := first[name]
		if !seen {
			first[name] = here
			continue
		}
		at, other := here, prev
		if r.Entities[here.ent].Flags&EfStd != 0 && r.Entities[prev.ent].Flags&EfStd == 0 {
			at, other = prev, here
		}
		ae := &r.Entities[at.ent]
		switch {
		case here.export && prev.export:
			r.errNote(ae.File, ae.Node, Binding{Ent: other.ent}, "also exported here", cCSymbolConflict, name, "is exported more than once")
		case prev.shape != here.shape:
			r.errNote(ae.File, ae.Node, Binding{Ent: other.ent}, "also declared here", cCSymbolConflict, name, "is declared with different C signatures")
		}
	}
}

func (r *Result) cSignatureShape(sig TypeID) string {
	n := r.Types.Node(sig)
	var sb strings.Builder
	sb.WriteByte('(')
	for _, a := range n.Args {
		sb.WriteString(r.cShape(a))
		sb.WriteByte(',')
	}
	if n.Flags&fnCVariadic != 0 {
		sb.WriteString("...")
	}
	sb.WriteString(")")
	sb.WriteString(r.cShape(n.Elem))
	return sb.String()
}

func (r *Result) cShape(t TypeID) string {
	return r.cShapeAt(t, 0)
}

func (r *Result) cShapeAt(t TypeID, depth int) string {
	tt := r.Types
	u := r.underlyingRepr(t)
	switch {
	case u == TyUnit || u == TyNever:
		return "void"
	case tt.IsFloat(u):
		return "f" + strconv.Itoa(tt.Width(u))
	case tt.IsInteger(u):
		return "i" + strconv.Itoa(tt.Width(u))
	case tt.Kind(u) == KPtr || tt.IsCFn(u):
		return "ptr"
	}
	if n := tt.Node(u); n.Kind == KNamed && depth < 8 && r.typeDecl(n.Ent).Form == FormRecord {
		info := r.typeDecl(n.Ent)
		var sb strings.Builder
		sb.WriteByte('{')
		for _, f := range r.fieldTypes(info, instSubst(info.Params, n.Args)) {
			sb.WriteString(r.cShapeAt(f, depth+1))
			sb.WriteByte(',')
		}
		sb.WriteByte('}')
		return sb.String()
	}
	return r.TypeString(u)
}

func (r *Result) bareTarget() bool { return r.target == Freestanding || r.noPlatform }
