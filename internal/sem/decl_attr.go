package sem

import "kigumi/internal/syntax"

// declareAttrs validates `@impl(...)` targets and `layout(...)` clauses and
// rejects every other attribute.
func (r *Result) declareAttrs() {
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.File == 0 || e.Flags&EfPoison != 0 {
			continue
		}
		t := r.tree(e.File)
		switch e.Kind {
		case EntType, EntAlias:
			s := typeDecl(t, e.Node)
			r.implAttrs(EntityID(id), s.Attrs)
			ug := r.extractGuarantee(e.File, s.Attrs)
			if e.Kind == EntType {
				info := r.typeDecl(EntityID(id))
				info.Metadata = r.metaAttrs(e.File, s.Attrs, true)
				for _, fld := range info.Fields {
					r.checkSchemas(e.File, r.field(fld).Metadata)
				}
				r.declareGuarantee(EntityID(id), ug)
			} else if ug != 0 {
				r.errAt(e.File, ug, cUnsafeGuaranteeWrongTarget)
			}
		case EntFn:
			s := fnDecl(t, e.Node)
			if r.Entities[e.Parent].Kind == EntInterface {
				r.rejectGuarantee(e.File, s.Attrs)
				if s.Params != 0 {
					for _, p := range t.Children(s.Params) {
						r.rejectGuarantee(e.File, param(t, p).Attrs)
					}
				}
				continue
			}
			info := r.Fn(EntityID(id))
			info.Metadata = r.metaAttrs(e.File, s.Attrs, false)
			r.rejectGuarantee(e.File, s.Attrs)
			for _, p := range info.Params {
				pAttrs := param(t, r.Entities[p].Node).Attrs
				r.local(p).Metadata = r.metaAttrs(e.File, pAttrs, false)
				r.rejectGuarantee(e.File, pAttrs)
			}
		case EntInterface:
			r.rejectAttrs(e.File, ifaceDecl(t, e.Node).Attrs)
			r.rejectGuarantee(e.File, ifaceDecl(t, e.Node).Attrs)
		case EntConst:
			s := constDecl(t, e.Node)
			r.constInfo(EntityID(id)).Metadata = r.metaAttrs(e.File, s.Attrs, false)
			r.rejectGuarantee(e.File, s.Attrs)
		}
	}
}

func (r *Result) implAttrs(id EntityID, list syntax.NodeID) {
	if list == 0 {
		return
	}
	e := &r.Entities[id]
	f := e.File
	t := r.tree(f)
	scope := r.declScope(id)
	for _, a := range t.Children(list) {
		if t.Kind(a) == syntax.UnsafeGuarantee {
			continue
		}
		name := pathText(t, syntax.NodeID(t.Nodes[a].Lhs), ".")
		if name != "impl" {
			// Dotted metadata is collected by metaAttrs.
			if len(t.PathToks(syntax.NodeID(t.Nodes[a].Lhs))) == 1 {
				r.errAt(f, a, cAttrUnknown, name)
			}
			continue
		}
		if e.Kind == EntAlias {
			r.errAt(f, a, cImplOnAlias)
			continue
		}
		for _, arg := range t.Children(syntax.NodeID(t.Nodes[a].Rhs)) {
			if ty, ok := r.implTarget(f, scope, arg); ok {
				r.typeDecl(id).Impls = append(r.typeDecl(id).Impls, ImplAssert{Iface: ty, Node: arg})
			}
		}
	}
}

// implTarget reads an `@impl` argument naming an interface instance.
func (r *Result) implTarget(f FileID, scope ScopeID, n syntax.NodeID) (TypeID, bool) {
	ent, args, ok := r.exprAsTypeRef(f, scope, n)
	if !ok {
		return 0, false
	}
	switch r.Entities[ent].Kind {
	case EntInterface:
		if !r.checkArity(f, n, ent, len(r.iface(ent).Params), len(args)) {
			return 0, false
		}
		return r.Types.Iface(ent, args), true
	case EntConstraint:
		r.errAt(f, n, cImplCompilerKnown, r.Entities[ent].Name)
	default:
		r.errAt(f, n, cImplNotInterface, r.Entities[ent].Name)
	}
	return 0, false
}
