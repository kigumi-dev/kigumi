package sem

import (
	"strings"

	"kigumi/internal/syntax"
)

// declareSignatures runs as pass 4.
func (r *Result) declareSignatures() {
	var fns []EntityID
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind == EntFn && e.File != 0 && e.Flags&EfPoison == 0 && r.Entities[e.Parent].Kind != EntInterface {
			fns = append(fns, EntityID(id))
		}
	}
	for _, id := range fns {
		r.declareFn(id)
	}
	for _, id := range fns {
		if r.Fn(id).Owner != 0 {
			r.registerMember(id)
		}
	}
	r.checkOverloadSets()
	r.declareNominalAttrs()
	r.declareAttrs()
	r.checkDeclarations()
}

func (r *Result) declareFn(id EntityID) {
	e := &r.Entities[id]
	f := e.File
	t := r.tree(f)
	s := fnDecl(t, e.Node)
	scope := r.declScope(id)
	info := r.Fn(id)
	var selfType TypeID
	if s.Recv != 0 {
		owner, ownerType := r.resolveReceiver(id, s.Recv, scope)
		info = r.Fn(id)
		info.Owner = owner
		selfType = ownerType
	}
	own := r.declareGenerics(f, id, s.Generics, scope)
	r.resolveConstraints(f, scope, s.Generics, own)
	r.Fn(id).TypeParams = append(r.Fn(id).TypeParams, own...)
	r.resolveSignature(id, scope, selfType)
	if s.Recv != 0 {
		if owner := r.Fn(id).Owner; owner != 0 {
			r.authorizeForeignMember(id, owner, selfType, s.Recv)
		}
	}
	r.declareWitnesses(id)
	r.declareConstParams(id)
	info = r.Fn(id)
	if (info.Abi != "" || info.Export != "") && len(r.constParamsOf(id)) > 0 {
		r.errAt(f, e.Node, cExportConstParam, e.Name)
	}
	if s.Mods&syntax.ModStrayBang != 0 {
		r.errAt(f, e.Node, cStrayBangReturn, r.TypeString(r.Types.Node(info.Sig).Elem))
	}
	mods := Effects(s.Mods)
	switch {
	case mods&EffAsync != 0 && mods&EffPure != 0:
		r.errAt(f, e.Node, cModsAsyncConflict, "pure")
	case mods&EffAsync != 0 && mods&EffUnsafe != 0:
		r.errAt(f, e.Node, cModsAsyncConflict, "unsafe")
	case mods&EffAsync != 0 && mods&EffNoalloc != 0:
		r.errAt(f, e.Node, cUnsupportedAsync)
	}
	if mods&EffAsync != 0 && s.Ret != 0 {
		r.checkAsyncTaskReturn(f, s.Ret, info.Sig)
	}
	if info.Export != "" && mods&EffAsync != 0 {
		r.errAt(f, e.Node, cExportAsync, e.Name)
	}
	if info.Abi != "" && mods&(EffPure|EffNoalloc|EffAsync) != 0 {
		r.errAt(f, e.Node, cForeignMods, effectsText(mods&(EffPure|EffNoalloc|EffAsync)))
	}
	if info.Naked && mods != 0 {
		r.errAt(f, e.Node, cNakedMods, effectsText(mods))
	}
	if info.Naked && s.Recv != 0 {
		r.errAt(f, e.Node, cNakedMethod)
	}
	r.checkInterruptSignature(f, e, info)
	if s.Mods&syntax.ModPureVar != 0 {
		r.errAt(f, e.Node, cEffectPolyNotReq)
	}
	if info.Abi != "" || info.Export != "" {
		for _, prefix := range []string{"kg_export_", "kg_byval_", "kg_call_"} {
			if strings.HasPrefix(e.Name, prefix) {
				r.errAt(f, e.Node, cCSymbolReserved, e.Name)
			}
		}
	}
	if info.Body == 0 && info.Abi == "" && e.Flags&EfStd == 0 {
		r.errAt(f, e.Node, cFnMissingBody, e.Name)
	}
	if info.Body != 0 && info.Abi != "" {
		r.errAt(f, e.Node, cExternBody, e.Name)
	}
	if info.Owner == 0 && info.SelfParam != 0 && s.Recv == 0 {
		r.errAt(f, r.Entities[info.SelfParam].Node, cSelfOutsideMember)
		info.Recv = RecvNone
	}
	if info.Owner != 0 && info.SelfParam != 0 && !t.View(s.Recv).AsTypePath().Args().Valid() && len(r.typeDecl(info.Owner).Params) > 0 {
		owner := r.Entities[info.Owner].Name
		r.errAt(f, s.Recv, cRecvGenericBinder, owner, owner, r.paramNames(r.typeDecl(info.Owner).Params), e.Name)
	}
	r.declareDefaults(id)
}
