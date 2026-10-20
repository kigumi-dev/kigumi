package mir

import "kigumi/internal/sem"

// genericParamLifetimeSource matches a bare-type-parameter return against the
// same parameter on info's own params (no per-call-site lifetime substitution).
func (c *borrowCheck) genericParamLifetimeSource(ent sem.EntityID, info *sem.FnInfo, args []LocalID, start, calleeBase int) (*borrowInfo, bool) {
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
}

func (c *borrowCheck) candidateInfo(args []LocalID, cand lifetimeCandidate) (*borrowInfo, bool) {
	if cand.arg >= len(args) {
		return nil, false
	}
	return c.flowInfo(flowArg{arg: args[cand.arg], contents: cand.contents})
}

// resolveLifetimeHolder drops candidates whose argument is a phantom lifetime
// with no tracked holder (e.g. a `['a]` tagging no actual `&'a` field).
func (c *borrowCheck) resolveLifetimeHolder(ent sem.EntityID, args []LocalID, candidates []lifetimeCandidate) (*borrowInfo, bool) {
	var tracked []lifetimeCandidate
	for _, cand := range candidates {
		if _, ok := c.candidateInfo(args, cand); ok {
			tracked = append(tracked, cand)
		}
	}
	if i, ok := c.resolveLifetimeCandidate(ent, tracked); ok {
		return c.candidateInfo(args, tracked[i])
	}
	return c.sameKindCandidate(args, tracked)
}

// resolveLifetimeCandidate picks the index of the sole candidate the callee's own body
// provably returns; a witness call has no body and falls through to sameKindCandidate.
func (c *borrowCheck) resolveLifetimeCandidate(ent sem.EntityID, candidates []lifetimeCandidate) (int, bool) {
	switch len(candidates) {
	case 0:
		return 0, false
	case 1:
		return 0, true
	}
	callee, ok := c.p.calleeBody(ent)
	if !ok {
		return 0, false
	}
	match, found := -1, false
	for i, cand := range candidates {
		if cand.callee >= len(callee.Params) || !calleeReturnsParam(callee, callee.Params[cand.callee]) {
			continue
		}
		if found {
			return 0, false
		}
		match, found = i, true
	}
	return match, found
}

// sameKindCandidate is the untraceable-body fallback: it unions every same-kind
// candidate's owners, since the callee could return any of them at runtime.
func (c *borrowCheck) sameKindCandidate(args []LocalID, candidates []lifetimeCandidate) (*borrowInfo, bool) {
	if len(candidates) == 0 {
		return nil, false
	}
	first, ok := c.candidateInfo(args, candidates[0])
	if !ok {
		return nil, false
	}
	merged := &borrowInfo{hasOwner: first.hasOwner, lifetime: first.lifetime, node: first.node}
	merged.owners = append(merged.owners, first.owners...)
	for _, cand := range candidates[1:] {
		b, ok := c.candidateInfo(args, cand)
		if !ok || b.hasOwner != first.hasOwner || (!first.hasOwner && b.lifetime != first.lifetime) {
			return nil, false
		}
		merged.owners = append(merged.owners, b.owners...)
	}
	return merged, true
}

// calleeReturnsParam reports whether every return point traces back to
// param through copies and moves only, so it can't loop through recursion.
func calleeReturnsParam(callee *Func, param LocalID) bool {
	found := false
	for b := range callee.Blocks {
		term := &callee.Blocks[b].Term
		if term.Op != TermReturn {
			continue
		}
		found = true
		if !calleeTracesToParam(callee, term.Args[0], param) {
			return false
		}
	}
	return found
}

func calleeTracesToParam(callee *Func, from, param LocalID) bool {
	seen := map[LocalID]bool{}
	for {
		if from == param {
			return true
		}
		if seen[from] {
			return false
		}
		seen[from] = true
		in := calleeDefinition(callee, from)
		if in == nil || (in.Op != OpCopy && in.Op != OpMove) {
			return false
		}
		from = in.Args[0]
	}
}

// calleeDefinition is definition's standalone counterpart for a Func other than
// the one being checked; no per-call cache since this only hits the rare multi-candidate callee.
func calleeDefinition(f *Func, l LocalID) *Inst {
	var found *Inst
	for b := range f.Blocks {
		for i := range f.Blocks[b].Insts {
			in := &f.Blocks[b].Insts[i]
			if in.Dst != l {
				continue
			}
			if !shapes[in.Op].result || f.Locals[in.Dst].Ent != 0 || found != nil {
				return nil
			}
			found = in
		}
	}
	return found
}
