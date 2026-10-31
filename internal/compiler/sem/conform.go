package sem

import "strings"

type conformKey struct {
	t, iface TypeID
	from     PackageID
}

type conformResult struct {
	ok        bool
	missing   string
	witnesses []EntityID
	// generic marks a missing failure that came from an undetermined
	// generic method, for the richer coercion diagnostic (E308).
	generic bool
}

// conforms decides structural conformance of t to an interface instance, as
// seen from package from.
func (r *Result) conforms(t, iface TypeID, from PackageID) conformResult {
	key := conformKey{t, iface, from}
	if res, ok := r.conformMemo[key]; ok {
		return res
	}
	res := r.computeConforms(t, iface, from)
	r.conformMemo[key] = res
	return res
}

// Nonzero means the `&mut` cell fixpoint closed over the witness table before resolveRefCells finished sealing it.
func (r *Result) ConformMissesAfterSeal() int { return len(r.conformMemo) - r.seal.conforms }

func (r *Result) computeConforms(t, iface TypeID, from PackageID) conformResult {
	if t == TyPoison || iface == TyPoison {
		return conformResult{ok: true}
	}
	subst := r.ifaceInstSubst(iface, t)
	info := r.iface(r.Types.Node(iface).Ent)
	var missing []string
	var witnesses []EntityID
	generic := false
	for _, req := range info.Reqs {
		w, why, whyGeneric, _ := r.findWitness(t, req, subst, from)
		if w == 0 {
			missing = append(missing, why)
			generic = generic || whyGeneric
			continue
		}
		if why := r.escapeMismatch(req, w); why != "" {
			missing = append(missing, why)
			continue
		}
		witnesses = append(witnesses, w)
	}
	if len(missing) > 0 {
		return conformResult{missing: strings.Join(missing, "; "), generic: generic}
	}
	return conformResult{ok: true, witnesses: witnesses}
}

// escapeMismatch rejects a witness that may keep a callback the requirement promises callers it
// won't: a witness may be more conservative than the requirement, never looser.
func (r *Result) escapeMismatch(req, w EntityID) string {
	rp, wp := r.Fn(req).Params, r.Fn(w).Params
	if len(rp) != len(wp) {
		return ""
	}
	for i := range rp {
		if r.nonEscapingParam(rp[i]) && !r.nonEscapingParam(wp[i]) {
			return r.noteText(cEscapingWitness.Template, r.Entities[w].Name, i+1)
		}
	}
	return ""
}

func (r *Result) ifaceInstSubst(iface, t TypeID) map[EntityID]TypeID {
	inode := r.Types.Node(iface)
	info := r.iface(inode.Ent)
	subst := map[EntityID]TypeID{info.SelfParam: t}
	for i, p := range info.Params {
		if i < len(inode.Args) {
			subst[p] = inode.Args[i]
		}
	}
	return subst
}

// findWitness looks for the method of t that satisfies req. A generic method wins only when unifying with req fixes
// every own type parameter uniquely; ownSubst carries that, nil when there is none.
func (r *Result) findWitness(t TypeID, req EntityID, subst map[EntityID]TypeID, from PackageID) (m EntityID, why string, generic bool, ownSubst map[EntityID]TypeID) {
	rinfo := r.Fn(req)
	name := r.Entities[req].Name
	want := r.Types.Subst(rinfo.Sig, subst)
	wantNode := r.Types.Node(want)
	sig := r.reqText(name, rinfo.Recv, want)
	set := r.memberSet(t, name)
	if set == 0 {
		return 0, "missing `" + sig + "`", false, nil
	}
	var first EntityID
	var firstOwn map[EntityID]TypeID
	for _, cand := range r.Overloads[set].Members {
		minfo := r.Fn(cand)
		if minfo.Recv == RecvNone && rinfo.Recv != RecvNone {
			why, generic = "`"+name+"` is an associated function, not a method", false
			continue
		}
		if minfo.Recv != RecvNone && rinfo.Recv == RecvNone {
			why, generic = "`"+name+"` is a method, the requirement is an associated function", false
			continue
		}
		if r.Entities[cand].Pkg != r.Entities[req].Pkg && !r.memberImportedBy(cand, from) {
			why, generic = "`"+name+"` is declared in package `"+r.Packages[r.Entities[cand].Pkg].Path+"`, which is not imported here", false
			continue
		}
		if !r.visibleFrom(r.Entities[cand].Vis, from) {
			why, generic = "`"+name+"` is "+r.visText(r.Entities[cand].Vis)+" in package `"+r.Packages[r.Entities[cand].Pkg].Path+"`", false
			continue
		}
		got := r.memberSig(t, cand)
		gotNode := r.Types.Node(got)
		var candOwn map[EntityID]TypeID
		if own := r.methodOwnParams(cand); len(own) > 0 {
			var undetermined []EntityID
			candOwn, undetermined = r.unifyMethodOwn(own, gotNode, wantNode)
			if len(undetermined) > 0 {
				why, generic = r.genericMethodWhy(cand, req, own, undetermined), true
				continue
			}
			if bad := r.ownConstraintWhy(cand, req, own, candOwn, from); bad != "" {
				why, generic = bad, true
				continue
			}
			got = r.Types.Subst(got, candOwn)
			gotNode = r.Types.Node(got)
		}
		switch {
		case minfo.Recv != rinfo.Recv:
			why, generic = "`"+name+"` takes `"+recvText(minfo.Recv)+"`, the requirement takes `"+recvText(rinfo.Recv)+"`", false
		case !sameCallableShape(r.Types, got, want):
			why, generic = "`"+name+"` has signature `"+r.reqText(name, minfo.Recv, got)+"`, the requirement is `"+sig+"`", false
		case (minfo.Declared^rinfo.Declared)&EffAsync != 0:
			why, generic = r.asyncWitnessWhy(cand, minfo.Declared&EffAsync != 0), false
		case (gotNode.Flags^wantNode.Flags)&uint16(EffUnsafe) != 0:
			why, generic = "`"+name+"` differs in `unsafe`", false
		case Effects(gotNode.Flags)&Effects(wantNode.Flags)&(EffPure|EffNoalloc) != Effects(wantNode.Flags)&(EffPure|EffNoalloc):
			why, generic = "`"+name+"` is not `"+effectsText(Effects(wantNode.Flags)&(EffPure|EffNoalloc))+"`", false
		default:
			if r.Entities[cand].Pkg == r.Entities[req].Pkg {
				r.noteWitness(req, cand)
				return cand, "", false, candOwn
			}
			if first == 0 {
				first, firstOwn = cand, candOwn
			}
		}
	}
	if first != 0 {
		r.noteWitness(req, first)
		return first, "", false, firstOwn
	}
	return 0, why, generic, nil
}
