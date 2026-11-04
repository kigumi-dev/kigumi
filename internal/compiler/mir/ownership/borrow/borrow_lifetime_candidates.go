package borrow

import (
	"kigumi/internal/compiler/sem"
)

// retLifetimeCandidates are the operands of a call passing info's receiver at args[recvIdx] and its own params after
// whose declared lifetime is retLt.
func (c *borrowCheck) retLifetimeCandidates(info *sem.FnInfo, retLt sem.EntityID, recvIdx int) []lifetimeCandidate {
	start := recvIdx
	calleeBase := 0
	if info.Recv != sem.RecvNone {
		start = recvIdx + 1
		calleeBase = 1
	}
	var candidates []lifetimeCandidate
	if info.Recv != sem.RecvNone {
		for _, p := range c.recvLifetimeParams(info) {
			if p == retLt {
				candidates = append(candidates, lifetimeCandidate{arg: recvIdx, callee: 0})
				break
			}
		}
		if info.SelfParam != 0 && c.p.R.Local(info.SelfParam).Lifetime == retLt {
			candidates = append(candidates, lifetimeCandidate{arg: recvIdx, callee: 0, recv: info.Recv})
		}
	}
	for i, prm := range info.Params {
		if c.paramLifetimeOf(info, prm) == retLt {
			lent := c.p.R.Types.Kind(c.p.R.Entity(prm).Type) == sem.KRef && c.p.R.Local(prm).Lifetime == 0
			candidates = append(candidates, lifetimeCandidate{arg: start + i, callee: calleeBase + i, contents: lent})
		}
	}
	return candidates
}
