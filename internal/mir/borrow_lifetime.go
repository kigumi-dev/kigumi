package mir

import "kigumi/internal/sem"

// lifetimeSource finds the existing holder a value that may carry a named lifetime
// should alias, matched by entity identity (no per-call-site lifetime substitution).
func (c *borrowCheck) lifetimeSource(in Inst) (*borrowInfo, bool) {
	switch in.Op {
	case OpRecord:
		return c.recordLifetimeSource(in)
	case OpCall:
		return c.callLifetimeSource(in)
	case OpCallValue:
		return c.callValueLifetimeSource(in)
	case OpBuiltin:
		if in.Str == "future" {
			return c.futureLifetimeSource(in)
		}
	}
	return nil, false
}

// futureLifetimeSource treats a future's captures (Args[1:]; Args[0] is the
// async fn item) like recordLifetimeSource treats a record's fields.
func (c *borrowCheck) futureLifetimeSource(in Inst) (*borrowInfo, bool) {
	return c.recordLifetimeSource(Inst{Dst: in.Dst, Args: in.Args[1:]})
}

// recordLifetimeSource requires every borrow argument to be the same holder; one
// built from two distinct borrows is left unresolved for checkInst to report.
func (c *borrowCheck) recordLifetimeSource(in Inst) (*borrowInfo, bool) {
	var found *borrowInfo
	for _, a := range in.Args {
		b, ok := c.holders[a]
		if !ok {
			continue
		}
		if found != nil && found != b {
			return nil, false
		}
		found = b
	}
	if found == nil {
		return nil, false
	}
	if !found.hasOwner {
		if declared := c.declaredLifetime(c.f.Locals[in.Dst].Type); declared == 0 || found.lifetime != declared {
			return nil, false
		}
	}
	return found, true
}

// callLifetimeSource matches the callee's return lifetime against its
// receiver and params, which OpCall always lowers as args[0] and args[1:].
func (c *borrowCheck) callLifetimeSource(in Inst) (*borrowInfo, bool) {
	if in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn {
		if b, ok := c.entityLifetimeSource(in.Ent, c.p.R.Fn(in.Ent), in.Args, 0); ok {
			return b, true
		}
	}
	return c.rejectUntrackedResult(in)
}

// callValueLifetimeSource handles OpCallValue: a witnessed interface requirement keeps
// Ent and lowers args[0] as the witness; a closure or fn pointer always falls to rejectUntrackedResult.
func (c *borrowCheck) callValueLifetimeSource(in Inst) (*borrowInfo, bool) {
	if in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn {
		if b, ok := c.entityLifetimeSource(in.Ent, c.p.R.Fn(in.Ent), in.Args, 1); ok {
			return b, true
		}
	}
	return c.rejectUntrackedResult(in)
}

// rejectUntrackedResult is the shared fallback for a call whose result could not
// be traced to an owner: silence is only safe when it could never carry a lifetime.
func (c *borrowCheck) rejectUntrackedResult(in Inst) (*borrowInfo, bool) {
	if c.resultMayCarryLifetime(c.f.Locals[in.Dst].Type) {
		code, msg := lifetimeUntrackedDiag("lifetime-untracked-call")
		c.report(in.Node, in.Dst, code, msg)
	}
	return nil, false
}

// resultMayCarryLifetime reports whether t needs a traceable owner: a bare
// `&T` carries no tag of its own, so it always does.
func (c *borrowCheck) resultMayCarryLifetime(t sem.TypeID) bool {
	return c.p.R.Types.Kind(t) == sem.KRef || c.declaredLifetime(t) != 0
}

// entityLifetimeSource matches info's declared return lifetime against its receiver
// at args[recvIdx] and its own params after; recvIdx is 1 for OpCallValue, else 0.
func (c *borrowCheck) entityLifetimeSource(ent sem.EntityID, info *sem.FnInfo, args []LocalID, recvIdx int) (*borrowInfo, bool) {
	start := recvIdx
	calleeBase := 0
	if info.Recv != sem.RecvNone {
		start = recvIdx + 1
		calleeBase = 1
	}
	if info.RetLifetime == 0 {
		return c.genericParamLifetimeSource(ent, info, args, start, calleeBase)
	}
	var candidates []lifetimeCandidate
	if info.Recv != sem.RecvNone {
		for _, p := range c.recvLifetimeParams(info) {
			if p == info.RetLifetime {
				candidates = append(candidates, lifetimeCandidate{arg: recvIdx, callee: 0})
				break
			}
		}
	}
	for i, prm := range info.Params {
		if c.paramLifetimeOf(info, prm) == info.RetLifetime {
			candidates = append(candidates, lifetimeCandidate{arg: start + i, callee: calleeBase + i})
		}
	}
	return c.resolveLifetimeHolder(ent, args, candidates)
}

// recvLifetimeParams is info's receiver's own lifetime-bearing type parameters:
// RecvParams when the binder carries a bound, else the owning type's or interface's own.
func (c *borrowCheck) recvLifetimeParams(info *sem.FnInfo) []sem.EntityID {
	if len(info.RecvParams) > 0 {
		return info.RecvParams
	}
	if info.Owner == 0 {
		return nil
	}
	switch c.p.R.Entity(info.Owner).Kind {
	case sem.EntType:
		return c.p.R.TypeDecl(info.Owner).Params
	case sem.EntInterface:
		return c.p.R.Iface(info.Owner).Params
	}
	return nil
}
