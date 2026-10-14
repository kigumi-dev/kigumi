package sem

// ProtoKind is "equals" or "hash" for a generated method of that name or the prelude's `hash`, "" otherwise.
func (r *Result) ProtoKind(fn EntityID) string {
	e := &r.Entities[fn]
	if fn == 0 || e.Kind != EntFn || (e.Name != "equals" && e.Name != "hash") {
		return ""
	}
	info := r.Fn(fn)
	switch {
	case info.Owner != 0:
		if f := r.tree(e.File).File; f == nil || !f.Generated || info.Body == 0 {
			return ""
		}
	case e.Name == "hash" && r.Packages[e.Pkg].Path == "std/prelude" && len(info.Params) == 1:
	default:
		return ""
	}
	return e.Name
}

// ProtocolDirect reports a direct record or closed ADT whose derived method only reads scalar leaves.
func (r *Result) ProtocolDirect(t TypeID, proto string) bool {
	return r.protocolDirect(t, proto, 0)
}

func (r *Result) protocolDirect(t TypeID, proto string, depth int) bool {
	if depth > 8 || t == 0 || !r.DirectAgg(t) || r.Types.Kind(t) != KNamed || r.IsFixedArray(t) {
		return false
	}
	n := r.Types.Node(t)
	info := r.typeDecl(n.Ent)
	if !r.derivedMember(info, proto) {
		return false
	}
	leaf := func(ts []TypeID) bool {
		for _, lt := range ts {
			if !r.protocolLeaf(lt, proto, depth+1) {
				return false
			}
		}
		return true
	}
	subst := instSubst(info.Params, n.Args)
	for _, v := range info.Variants {
		if !leaf(r.payloadTypes(v, subst)) {
			return false
		}
	}
	return leaf(r.fieldTypes(info, subst)) && (len(info.Fields) > 0 || len(info.Variants) > 0)
}

func (r *Result) protocolLeaf(t TypeID, proto string, depth int) bool {
	if r.RegScalar(t) {
		return true
	}
	return r.protocolDirect(t, proto, depth)
}

// The member must be the generated one: a method the author wrote keeps its own meaning.
func (r *Result) derivedMember(info *TypeDeclInfo, proto string) bool {
	set, ok := info.Members[proto]
	if !ok || len(r.Overloads[set].Members) != 1 {
		return false
	}
	return r.ProtoKind(r.Overloads[set].Members[0]) == proto
}

func (r *Result) ProtoSelf(fn EntityID) TypeID {
	info := r.Fn(fn)
	if info.Owner != 0 {
		return r.Entities[info.SelfParam].Type
	}
	if len(info.TypeParams) == 1 {
		return r.Types.Param(info.TypeParams[0])
	}
	return 0
}

// Such a method gets a synthesized body that allocates nothing.
func (r *Result) protoNoalloc(fn EntityID) bool {
	info := r.Fn(fn)
	kind := r.ProtoKind(fn)
	return kind != "" && info.Owner != 0 && len(info.TypeParams) == 0 && r.ProtocolDirect(r.Entities[info.SelfParam].Type, kind)
}

// A derived `equals` is noalloc when its field types are; a derived `hash` boxes its u64 result.
func (r *Result) derivedInferred(fn EntityID) bool {
	if r.Entities[fn].Kind != EntFn {
		return false
	}
	info := r.Fn(fn)
	return info.Owner != 0 && len(info.TypeParams) == 0 && r.ProtoKind(fn) == "equals" && !r.protoNoalloc(fn)
}
