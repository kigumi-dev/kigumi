package sem

import "kigumi/internal/syntax"

// resolveSignature creates a function or interface requirement's parameter
// entities and KFn signature; selfType is 0 for free functions.
func (r *Result) resolveSignature(id EntityID, scope ScopeID, selfType TypeID) {
	e := &r.Entities[id]
	f := e.File
	t := r.tree(f)
	s := fnDecl(t, e.Node)
	info := r.Fn(id)
	var params []TypeID
	var granted []bool
	variadic := false
	cvariadic := false
	list := t.List(s.Params)
	for i, p := range list.EnumerateIDs() {
		ps := param(t, p)
		pt := t.View(p).Tok()
		name := pt.Text()
		pe := Entity{Kind: EntParam, Name: name, Pkg: e.Pkg, File: f, Node: p, Tok: pt.Index(), Parent: id}
		if ps.Flags&syntax.FlagSelf != 0 {
			if i != 0 && s.Recv != 0 {
				r.errAt(f, p, cSelfPosition)
			}
			pe.Flags |= EfSelf
			switch {
			case ps.Flags&syntax.FlagMove != 0:
				pe.Flags |= EfMove
				info.Recv = RecvMove
			case ps.Flags&syntax.FlagMut != 0:
				pe.Flags |= EfMut
				info.Recv = RecvMut
			default:
				info.Recv = RecvSelf
			}
			pe.Type = selfType
			if selfType == 0 {
				pe.Type = TyPoison
			}
			sid := r.newEntity(pe)
			r.Entities[sid].Detail = r.addLocal(LocalInfo{Scope: scope})
			r.Files[f].Defs[p] = sid
			r.define(scope, "self", Binding{Ent: sid})
			info.SelfParam = sid
			continue
		}
		if ps.Flags&syntax.FlagVariadic != 0 && ps.Type == 0 {
			switch {
			case info.Abi == "":
				r.errAt(f, p, cCVariadicOutsideExtern)
			case i != list.Len()-1:
				r.errAt(f, p, cCVariadicNotLast)
			}
			cvariadic = true
			continue
		}
		typ := r.resolveType(f, scope, ps.Type, posParam)
		if info.DefaultOf != 0 && !r.isCopy(typ) && !r.Types.RefMut(typ) {
			typ = r.Types.Ref(typ, false)
		}
		grant := false
		if ps.Flags&syntax.FlagEscaping != 0 {
			typ, grant = r.escapingParamType(f, p, typ)
		}
		typ = r.paramEscapeType(id, info, typ)
		if grant || r.paramIsEscaping(id, info, typ) {
			pe.Flags |= EfEscaping
		}
		if ps.Flags&syntax.FlagVariadic != 0 {
			if i != list.Len()-1 {
				r.errAt(f, p, cVariadicNotLast)
			}
			variadic = true
			pe.Flags |= EfVariadic
			typ = r.Types.Named(r.Types.arrayEnt, []TypeID{r.Types.WithoutEscaping(typ)})
		}
		if ps.Flags&syntax.FlagMut != 0 {
			pe.Flags |= EfMut
		}
		if name == "nil" {
			r.errAt(f, p, cNilName)
		}
		pe.Type = typ
		pid := r.newEntity(pe)
		r.Entities[pid].Detail = r.addLocal(LocalInfo{Scope: scope, Lifetime: r.explicitLifetimeOf(f, scope, ps.Type)})
		r.Files[f].Defs[p] = pid
		if _, dup := r.define(scope, name, Binding{Ent: pid}); dup {
			r.errAt(f, p, cParamDuplicate, name)
		}
		info.Params = append(info.Params, pid)
		params = append(params, typ)
		granted = append(granted, pe.Flags&EfEscaping != 0)
	}
	ret := TyUnit
	switch {
	case s.Ret != 0:
		ret = r.resolveReturnType(f, scope, id, selfType, info.Params, s.Ret)
	case s.Mods&syntax.ModBareErr != 0:
		ret = r.unitResult()
	}
	info = r.Fn(id)
	switch {
	case info.Abi != "" || info.Naked:
		info.Sig = r.Types.FnCPtr(params, ret, cvariadic)
	case info.EffectPoly:
		info.Sig = r.Types.FnEffectPoly(params, ret, info.Declared&(EffPure|EffNoalloc|EffUnsafe), variadic)
	default:
		info.Sig = r.Types.Fn(params, ret, info.Declared&(EffPure|EffNoalloc|EffUnsafe), variadic)
	}
	if info.Abi == "" && !info.Naked {
		info.Sig = r.Types.WithEscParams(info.Sig, r.escapingMarks(params, granted))
	}
	r.Entities[id].Type = info.Sig
	if info.Abi != "" || info.Export != "" {
		r.checkAbiTypes(f, e.Node, s, params, ret, info.Abi != "")
	}
}
