package sem

import "kigumi/internal/compiler/syntax"

func (r *Result) collectType(f FileID, d syntax.NodeID, foreign bool) {
	t := r.tree(f)
	s := typeDecl(t, d)
	pkg := r.packageOf(f)
	if uni := r.lookupUniverse(t.TokText(s.Name)); uni != 0 && r.Packages[pkg].Std && r.Entities[uni].Kind == EntType && s.Body == 0 {
		r.Files[f].Defs[d] = uni
		r.define(r.Packages[pkg].Scope, t.TokText(s.Name), Binding{Ent: uni})
		return
	}
	kind := EntType
	info := TypeDeclInfo{Members: map[string]OverloadSetID{}, Ops: map[string]OverloadSetID{}, Home: pkg}
	switch t.Kind(s.Body) {
	case syntax.AliasBody:
		kind = EntAlias
	case syntax.DefinedBody:
		info.Form = FormDefined
	case syntax.RecordBody:
		info.Form = FormRecord
	case syntax.ResourceBody:
		info.Form = FormResource
	case syntax.AdtBody:
		info.Form = FormAdt
	default:
		info.Form = FormOpaque
		if !r.Packages[pkg].Std && !foreign {
			r.errAt(f, d, cOpaqueOutsideStd, t.TokText(s.Name))
		}
	}
	id := r.declareTop(f, d, s.Name, kind, r.resolveVis(f, s.Vis, pkg))
	if id == 0 {
		return
	}
	if kind == EntType {
		r.Entities[id].Detail = r.addTypeDecl(info)
	}
	if r.Packages[pkg].Std {
		r.Entities[id].Flags |= EfStd
	}
}
