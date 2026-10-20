package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// lifetimeSource finds the existing holder a value that may carry a named lifetime
// should alias, matched by entity identity (no per-call-site lifetime substitution).
func (c *borrowCheck) lifetimeSource(in Inst) (*borrowInfo, bool) {
	switch in.Op {
	case OpRecord, OpVariant:
		return c.recordLifetimeSource(in)
	case OpCall:
		return c.callLifetimeSource(in)
	case OpCallValue:
		return c.callValueLifetimeSource(in)
	case OpFutureNew:
		return c.futureLifetimeSource(in)
	}
	return nil, false
}

// futureLifetimeSource treats a future's captures (Args[1:]; Args[0] is the
// async fn item) like recordLifetimeSource treats a record's fields.
func (c *borrowCheck) futureLifetimeSource(in Inst) (*borrowInfo, bool) {
	return c.recordLifetimeSource(Inst{Dst: in.Dst, Args: in.Args[1:]})
}

// recordLifetimeSource shares the one holder the borrow arguments name; a record or variant built
// from several merges them, unless one is a parameter's borrow under a lifetime the type lacks.
func (c *borrowCheck) recordLifetimeSource(in Inst) (*borrowInfo, bool) {
	var found []*borrowInfo
	for _, a := range in.Args {
		if b, ok := c.holders[a]; ok && !slices.Contains(found, b) {
			found = append(found, b)
		}
	}
	if len(found) == 0 || len(found) > 1 && in.Op != OpRecord && in.Op != OpVariant {
		return nil, false
	}
	for _, b := range found {
		if !c.fitsDeclared(b, in.Dst) {
			return nil, false
		}
	}
	if len(found) == 1 {
		return found[0], true
	}
	s := c.sink(in.Dst, in.Node)
	for _, b := range found {
		c.merge(s, b)
	}
	return s, true
}

func (c *borrowCheck) fitsDeclared(b *borrowInfo, dst LocalID) bool {
	if b.hasOwner || b.empty || c.closureLifetime(b) {
		return true
	}
	declared := c.declaredLifetime(c.f.Locals[dst].Type)
	return declared != 0 && b.lifetime == declared
}

// callLifetimeSource matches the callee's return lifetime against its
// receiver and params, which OpCall always lowers as args[0] and args[1:].
func (c *borrowCheck) callLifetimeSource(in Inst) (*borrowInfo, bool) {
	if in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn {
		if b, ok := c.genericFlowSource(in, 0); ok {
			return b, true
		}
		if b, ok := c.entityLifetimeSource(in.Ent, c.p.R.Fn(in.Ent), in.Args, 0); ok {
			return b, true
		}
		if b, ok := c.futureHandleSource(in); ok {
			return b, true
		}
	}
	return c.rejectUntrackedResult(in)
}

// futureHandleSource keeps a future's borrows in the handle a call builds around it
// (`spawn(f) -> Task[T]`); a bare `T` result is the future's output, not the future.
func (c *borrowCheck) futureHandleSource(in Inst) (*borrowInfo, bool) {
	info := c.p.R.Fn(in.Ent)
	ret := c.p.R.Types.Node(info.Sig).Elem
	if c.p.R.Types.Kind(ret) == sem.KParam {
		return nil, false
	}
	want := c.typeParamsOf(ret)
	for _, s := range c.callSlots(info, in.Args, 0) {
		t := c.f.Locals[s.arg].Type
		if c.p.R.Types.Kind(t) != sem.KNamed || c.p.R.Types.Node(t).Ent != c.p.R.Types.FutureEnt() || !sharesParam(s.params, want) {
			continue
		}
		if b, ok := c.holders[s.arg]; ok {
			return b, true
		}
	}
	return nil, false
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
	retLt := c.fnRetLifetime(info)
	if retLt == 0 {
		return c.genericParamLifetimeSource(ent, info, args, start, calleeBase)
	}
	var candidates []lifetimeCandidate
	if info.Recv != sem.RecvNone {
		for _, p := range c.recvLifetimeParams(info) {
			if p == retLt {
				candidates = append(candidates, lifetimeCandidate{arg: recvIdx, callee: 0})
				break
			}
		}
	}
	for i, prm := range info.Params {
		if c.paramLifetimeOf(info, prm) == retLt {
			lent := c.p.R.Types.Kind(c.p.R.Entity(prm).Type) == sem.KRef && c.p.R.Local(prm).Lifetime == 0
			candidates = append(candidates, lifetimeCandidate{arg: start + i, callee: calleeBase + i, contents: lent})
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
