package sem

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// resolveReturnType resolves a bare, untagged `&T`/`&mut T` return to the
// sole `&'a` parameter's lifetime when exactly one exists.
func (r *Result) resolveReturnType(f FileID, scope ScopeID, id EntityID, selfType TypeID, params []EntityID, n syntax.NodeID) TypeID {
	cand := r.returnElisionCandidate(r.Fn(id), selfType, params)
	t := r.tree(f)
	if t.Kind(n) == syntax.TypeRef {
		ref := t.View(n).AsTypeRef()
		if cand == 0 && ref.Op().Kind() != token.Lifetime {
			cand = r.elidedRefLifetime(id, selfType, params)
		}
		if cand != 0 && ref.Op().Kind() != token.Lifetime {
			r.Fn(id).RetLifetime = cand
			elem := r.resolveType(f, scope, ref.Pointee().ID(), posTypeArg)
			return r.Types.Ref(elem, ref.Flags()&syntax.FlagMut != 0)
		}
	}
	ret := r.resolveType(f, scope, n, posReturn)
	r.Fn(id).RetLifetime = r.explicitLifetimeOf(f, scope, n)
	return ret
}

// returnElisionCandidate returns the sole `&'a` source a bare returned
// reference could take, or 0 if there is none or more than one.
func (r *Result) returnElisionCandidate(info *FnInfo, selfType TypeID, params []EntityID) EntityID {
	cand, count := EntityID(0), 0
	if info.Recv != RecvNone && info.Recv != RecvMove {
		var recvLt EntityID
		for _, p := range r.Types.Params(selfType) {
			if !r.typeParam(p).IsLifetime {
				continue
			}
			if recvLt != 0 {
				recvLt = 0
				break
			}
			recvLt = p
		}
		if recvLt != 0 {
			cand, count = recvLt, 1
		}
	}
	for _, p := range params {
		if lt := r.local(p).Lifetime; lt != 0 {
			count++
			cand = lt
		}
	}
	if count == 1 {
		return cand
	}
	return 0
}

// adtArgs gives unspecified generic arguments fresh variables, lifetimes too so a spread unifies structurally like
// instantiateFn; an untouched lifetime takes lifetimeFallback.
func (c *checker) adtArgs(n syntax.NodeID, adt EntityID, args []TypeID) []TypeID {
	params := c.r.typeDecl(adt).Params
	if len(args) != len(params) {
		args = make([]TypeID, len(params))
		for i := range params {
			if c.r.typeParam(params[i]).IsLifetime {
				args[i] = c.freshLifetime(n, params[i])
				continue
			}
			args[i] = c.vars.fresh(n, c.r.Entities[params[i]].Name)
		}
	}
	subst := make(map[EntityID]TypeID, len(params))
	for i, p := range params {
		subst[p] = args[i]
	}
	c.deferConstraints(n, params, args, subst)
	return args
}

// elidedRefLifetime gives the sole untagged reference source (the receiver
// counted first) a lifetime of its own when nothing is tagged, 0 otherwise.
func (r *Result) elidedRefLifetime(id EntityID, selfType TypeID, params []EntityID) EntityID {
	info := r.Fn(id)
	var src EntityID
	count := 0
	if info.Recv == RecvSelf || info.Recv == RecvMut {
		for _, p := range r.Types.Params(selfType) {
			if r.typeParam(p).IsLifetime {
				return 0
			}
		}
		src, count = info.SelfParam, 1
	}
	for _, p := range params {
		if r.local(p).Lifetime != 0 {
			return 0
		}
		if r.Types.Kind(r.Entities[p].Type) == KRef {
			src = p
			count++
		}
	}
	if count != 1 || src == 0 {
		return 0
	}
	lt := r.localLifetime(id, "'_")
	r.local(src).Lifetime = lt
	return lt
}
