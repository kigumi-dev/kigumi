package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// refKeptValues are the flows into slot i's place of what a call to e lends that the callee may keep there by its
// refKeeps bound to the callables the call passes, unless have already carries one.
func (c *borrowCheck) refKeptValues(e sem.EntityID, args []midir.LocalID, slots []callSlot, i int, have []flowArg) []flowArg {
	s := c.calleeRefKeeps(e, c.refBindSite(e, args))
	if !s.keepsAny() {
		return nil
	}
	info := c.p.R.Fn(e)
	var out []flowArg
	for j := range slots {
		for _, kind := range []refKind{refOwn, refHeld} {
			if !s.storesInto(i, j, kind) {
				continue
			}
			v := c.keptArg(info, slots, j, kind, s.keepsExcl(j))
			if !hasFlow(have, v) && !hasFlow(out, v) {
				out = append(out, v)
			}
		}
	}
	return out
}

func hasFlow(fs []flowArg, v flowArg) bool {
	return slices.ContainsFunc(fs, func(h flowArg) bool {
		return h.arg == v.arg && h.contents == v.contents && (h.tie == tieNone) == (v.tie == tieNone)
	})
}

// refKeptTie is the borrows of what call in lends that the callee may keep in a result holding a borrow; a Future or
// Task result keeps its arguments through summaryTie already.
func (c *borrowCheck) refKeptTie(in midir.Inst) []*borrowInfo {
	if !in.Op.Yields() || !c.facts().HoldsBorrow(c.f.Locals[in.Dst].Type) {
		return nil
	}
	s := c.calleeRefKeeps(in.Ent, c.refBindSite(in.Ent, in.Args))
	if !s.keepsAny() {
		return nil
	}
	info := c.p.R.Fn(in.Ent)
	slots := c.callSlots(info, in.Args, 0)
	var out []*borrowInfo
	for j := range slots {
		for _, kind := range []refKind{refOwn, refHeld} {
			if !s.keepsResult(j, kind) {
				continue
			}
			if b, ok := c.tieInfo(c.keptArg(info, slots, j, kind, s.keepsExcl(j)), false, false); ok {
				out = append(out, b)
			}
		}
	}
	return out
}

// keptArg is the flow of what slot j lends: own, a reborrow of a receiver's place (or temporary) or of what a reference
// names, shared unless kept exclusive; held, what its referent or value holds.
func (c *borrowCheck) keptArg(info *sem.FnInfo, slots []callSlot, j int, kind refKind, excl bool) flowArg {
	a := slots[j].arg
	recv := j == 0 && info.Recv != sem.RecvNone && slots[j].exact == 0
	switch {
	case recv && info.Recv == sem.RecvMove, !recv && c.p.R.Types.Kind(slots[j].exact) != sem.KRef:
		return flowArg{arg: a, param: j}
	case kind == refHeld:
		return flowArg{arg: a, contents: true, param: j}
	case recv:
		return flowArg{arg: a, tie: tieRecvKept, shared: !excl, param: j}
	}
	return flowArg{arg: a, tie: tieRef, shared: !excl, param: j}
}

// keptRecvInfo is a reborrow of a receiver's place, under a statement's temporary when it has no named root.
func (c *borrowCheck) keptRecvInfo(a midir.LocalID, mut bool) (*borrowInfo, bool) {
	if b, ok := c.receiverPlaceInfo(a, mut); ok {
		return b, true
	}
	if d := c.definition(a); d != nil && d.Op == midir.OpNewCell {
		a = d.Args[0]
	}
	root, path, exact, ok := c.tempPlace(a)
	if !ok || root == 0 {
		return nil, false
	}
	return &borrowInfo{owners: []ownerRef{{owner: root, mut: mut, path: path, exact: exact, temp: true}}, hasOwner: true, node: c.callNode}, true
}
