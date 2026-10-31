package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// genericParamLifetimeSource matches a bare-type-parameter return against the
// same parameter on info's own params (no per-call-site lifetime substitution).
func (c *borrowCheck) genericParamLifetimeSource(ent sem.EntityID, info *sem.FnInfo, args []midir.LocalID, start, calleeBase int) (*borrowInfo, bool) {
	sig := c.p.R.Types.Node(info.Sig)
	if c.p.R.Types.Kind(sig.Elem) != sem.KParam {
		return nil, false
	}
	ret := c.p.R.Types.Node(sig.Elem).Ent
	var candidates []lifetimeCandidate
	for i, pt := range sig.Args {
		if c.p.R.Types.Kind(pt) == sem.KParam && c.p.R.Types.Node(pt).Ent == ret {
			candidates = append(candidates, lifetimeCandidate{arg: start + i, callee: calleeBase + i})
		}
	}
	return c.resolveLifetimeHolder(ent, args, candidates)
}

// lifetimeCandidate pairs a caller-side argument with its matching local in
// the callee's own body, for resolveLifetimeCandidate's body trace.
type lifetimeCandidate struct {
	arg      int
	callee   int
	contents bool
	// recv is the receiver mode of an elided receiver lifetime, RecvNone otherwise.
	recv sem.RecvKind
}

func (c *borrowCheck) candidateInfo(args []midir.LocalID, cand lifetimeCandidate) (*borrowInfo, bool) {
	if cand.arg >= len(args) {
		return nil, false
	}
	if b, ok := c.flowInfo(flowArg{arg: args[cand.arg], contents: cand.contents}); ok || cand.recv == sem.RecvNone {
		return b, ok
	}
	return c.receiverPlaceInfo(args[cand.arg], cand.recv == sem.RecvMut)
}

// resolveLifetimeHolder drops candidates whose argument is a phantom lifetime
// with no tracked holder (e.g. a `['a]` tagging no actual `&'a` field).
func (c *borrowCheck) resolveLifetimeHolder(ent sem.EntityID, args []midir.LocalID, candidates []lifetimeCandidate) (*borrowInfo, bool) {
	var tracked []lifetimeCandidate
	for _, cand := range candidates {
		if _, ok := c.candidateInfo(args, cand); ok {
			tracked = append(tracked, cand)
		}
	}
	if i, one := c.resolveLifetimeCandidate(ent, tracked); one {
		return c.tiedInfo(args, tracked[i])
	}
	return c.sameKindCandidate(args, tracked)
}

// resolveLifetimeCandidate picks the index of the sole candidate the callee's own body provably
// returns unchanged; a witness call or a writable returned param falls through to sameKindCandidate.
func (c *borrowCheck) resolveLifetimeCandidate(ent sem.EntityID, candidates []lifetimeCandidate) (int, bool) {
	switch len(candidates) {
	case 0:
		return 0, false
	case 1:
		return 0, true
	}
	callee, ok := c.calleeBody(ent)
	if !ok {
		return 0, false
	}
	match, found := -1, false
	for i, cand := range candidates {
		if cand.callee >= len(callee.Params) || !calleeReturnsParam(callee, callee.Params[cand.callee]) {
			continue
		}
		if c.writableParam(callee.Params[cand.callee], callee) {
			return 0, false
		}
		if found {
			return 0, false
		}
		match, found = i, true
	}
	return match, found
}

// sameKindCandidate is the untraceable-body fallback: it unions every candidate's owners, since the callee could
// return any of them; borrows a parameter lends join only borrows held under that same parameter lifetime.
func (c *borrowCheck) sameKindCandidate(args []midir.LocalID, candidates []lifetimeCandidate) (*borrowInfo, bool) {
	var owned, lent []*borrowInfo
	for _, cand := range candidates {
		b, ok := c.tiedInfo(args, cand)
		switch {
		case !ok:
			return nil, false
		case b.hasOwner:
			owned = append(owned, b)
		default:
			lent = append(lent, b)
		}
	}
	if len(owned) == 0 && len(lent) == 0 || slices.ContainsFunc(lent, func(b *borrowInfo) bool { return b.lifetime != lent[0].lifetime }) {
		return nil, false
	}
	if len(owned) == 0 {
		merged := &borrowInfo{lifetime: lent[0].lifetime, node: lent[0].node}
		for _, b := range lent {
			merged.owners = append(merged.owners, b.owners...)
		}
		return merged, true
	}
	if len(lent) > 0 && (lent[0].lifetime == 0 || slices.ContainsFunc(owned, func(b *borrowInfo) bool { return referentLifetime(b) != lent[0].lifetime })) {
		c.noteMixed(owned)
		return nil, false
	}
	merged := &borrowInfo{hasOwner: true, node: owned[0].node}
	for _, b := range owned {
		merged.owners = append(merged.owners, b.owners...)
	}
	c.unions = append(c.unions, unionSite{merged, owned})
	return merged, true
}

// calleeReturnsParam reports whether every return point traces back to
// param through copies and moves only, so it can't loop through recursion.
func calleeReturnsParam(callee *midir.Func, param midir.LocalID) bool {
	found := false
	for b := range callee.Blocks {
		term := &callee.Blocks[b].Term
		if term.Op != midir.TermReturn {
			continue
		}
		found = true
		if !calleeTracesToParam(callee, term.Args[0], param) {
			return false
		}
	}
	return found
}

func calleeTracesToParam(callee *midir.Func, from, param midir.LocalID) bool {
	seen := map[midir.LocalID]bool{}
	for {
		if from == param {
			return true
		}
		if seen[from] {
			return false
		}
		seen[from] = true
		in := calleeDefinition(callee, from)
		if in == nil || (in.Op != midir.OpCopy && in.Op != midir.OpMove) {
			return false
		}
		from = in.Args[0]
	}
}

// calleeDefinition is definition's standalone counterpart for a Func other than
// the one being checked; no per-call cache since this only hits the rare multi-candidate callee.
func calleeDefinition(f *midir.Func, l midir.LocalID) *midir.Inst {
	var found *midir.Inst
	for b := range f.Blocks {
		for i := range f.Blocks[b].Insts {
			in := &f.Blocks[b].Insts[i]
			if in.Dst != l {
				continue
			}
			if !in.Op.Yields() || f.Locals[in.Dst].Ent != 0 || found != nil {
				return nil
			}
			found = in
		}
	}
	return found
}

func (c *borrowCheck) writableParam(prm midir.LocalID, callee *midir.Func) bool {
	ent := callee.Locals[prm].Ent
	return ent != 0 && c.p.R.Entity(ent).Flags&(sem.EfMut|sem.EfMove) != 0
}
