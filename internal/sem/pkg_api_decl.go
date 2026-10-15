package sem

import (
	"fmt"
	"strings"
)

func (r *Result) apiDecl(id EntityID) (APIDecl, bool) {
	e := &r.Entities[id]
	d := APIDecl{Name: r.apiName(id), Kind: e.Kind.String(), Vis: r.apiVis(e.Vis)}
	switch e.Kind {
	case EntType:
		r.apiType(&d, id)
	case EntAlias:
		d.Sig = r.TypeString(e.Type)
		d.Generics = r.apiParams(r.aliasParamList(id))
	case EntInterface:
		r.apiInterface(&d, id)
	case EntFn:
		if r.Entities[e.Parent].Kind == EntInterface {
			return d, false
		}
		r.apiFn(&d, id)
	case EntConst:
		d.Sig = r.TypeString(e.Type)
		if v, ok := r.ConstValueOf(id); ok {
			d.Value = fmt.Sprintf("%v", v)
		}
	case EntStatic:
		st := r.Static(id)
		d.Sig = r.TypeString(e.Type)
		d.Abi, d.LinkName, d.Section = st.Abi, st.LinkName, st.Section
		d.Layout = fmt.Sprintf("mut=%v", st.Mut)
	case EntContextKey:
		d.Sig = r.TypeString(e.Type)
	case EntImport:
		if e.Flags&EfReexport == 0 {
			return d, false
		}
		d.Target = r.apiName(e.Target)
	default:
		return d, false
	}
	return d, true
}

func (r *Result) apiName(id EntityID) string {
	if id == 0 {
		return ""
	}
	e := &r.Entities[id]
	name := e.Name
	if e.Kind == EntFn {
		if owner := r.Fn(id).Owner; owner != 0 {
			name = r.Entities[owner].Name + "." + name
		}
	}
	if e.Kind == EntPackage {
		return name
	}
	return r.Packages[e.Pkg].Path + "." + name
}

func (r *Result) apiVis(v Visibility) string {
	if v.Level == VisIn || v.Level == VisSuper {
		return v.Level.String() + ":" + r.Packages[v.Scope].Path
	}
	return v.Level.String()
}

func (r *Result) aliasParamList(id EntityID) []EntityID { return r.aliasParams[id] }

func (r *Result) apiParams(ids []EntityID) []string {
	var out []string
	for _, p := range ids {
		var sb strings.Builder
		sb.WriteString(r.Entities[p].Name)
		info := r.typeParam(p)
		switch {
		case info.IsLifetime:
			sb.WriteString(":lifetime")
		case info.IsConst:
			sb.WriteString(":const " + r.TypeString(info.ConstType))
		}
		for _, c := range info.Constraints {
			fmt.Fprintf(&sb, " %d:%s", c.Kind, r.TypeString(c.Type))
		}
		out = append(out, sb.String())
	}
	return out
}

func (r *Result) apiInterface(d *APIDecl, id EntityID) {
	info := r.iface(id)
	d.Generics = r.apiParams(info.Params)
	d.Layout = fmt.Sprintf("objectSafe=%d derivable=%v", info.ObjectSafe, info.Derivable)
	for _, req := range info.Reqs {
		fn := r.Fn(req)
		d.Reqs = append(d.Reqs, fmt.Sprintf("%s %s %s poly=%v", r.Entities[req].Name, r.TypeString(fn.Sig), effectBits(fn.Declared), fn.EffectPoly))
	}
}

func effectBits(e Effects) string {
	var parts []string
	for _, f := range []struct {
		bit  Effects
		name string
	}{{EffPure, "pure"}, {EffNoalloc, "noalloc"}, {EffAsync, "async"}, {EffUnsafe, "unsafe"}} {
		if e&f.bit != 0 {
			parts = append(parts, f.name)
		}
	}
	return strings.Join(parts, ",")
}
