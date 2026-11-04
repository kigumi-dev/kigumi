package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) withRefKeeps(k *keptCtx, in midir.Inst, res uint64, lent func(int) uint64) (uint64, func(int) uint64) {
	r, into := c.refKeepCall(k, in)
	return res | r, func(i int) uint64 { return lent(i) | into(i) }
}

// calleeUse is one callee a call may run, with each of its argument slots' operand: an index into the call's
// operands, or via for a value the called callable holds itself (a capture, a method value's receiver).
type calleeUse struct {
	s     *refSummary
	slots []useSlot
}

type useSlot struct {
	j, arg  int
	via     bool
	applied bool
}

// refKeepCall is what call in keeps of what its operands lend, in its result and per operand that operand's place, by
// refCallees' summaries and each operand's refOrigin.
func (c *borrowCheck) refKeepCall(k *keptCtx, in midir.Inst) (uint64, func(int) uint64) {
	var res uint64
	into := map[int]uint64{}
	uses := c.refCallees(k, in)
	if len(uses) == 0 {
		return 0, func(int) uint64 { return 0 }
	}
	orig := c.refOrigins(k)
	every := c.windowBits(k)
	pick := func(sl useSlot, kind refKind) uint64 {
		o := orig[in.Args[sl.arg]]
		switch {
		case o.unknown, sl.applied && kind == refOwn:
			return every
		case kind == refOwn && !sl.via:
			return o.own
		}
		return o.held
	}
	for _, u := range uses {
		target := map[int]int{}
		for _, sl := range u.slots {
			target[sl.j] = sl.arg
		}
		for _, sl := range u.slots {
			for _, kind := range []refKind{refOwn, refHeld} {
				var bits uint64
				if u.s.keepsResult(sl.j, kind) {
					bits = pick(sl, kind)
					res |= bits
				}
				for t, arg := range target {
					if u.s.storesInto(t, sl.j, kind) {
						into[arg] |= pick(sl, kind)
					}
				}
			}
			if a := in.Args[sl.arg]; u.s.keepsExcl(sl.j) && (c.p.R.Types.RefMut(k.g.Locals[a].Type) || k.g.Locals[a].Cell) {
				k.excl |= pick(sl, refOwn)
			}
		}
	}
	return res, func(i int) uint64 { return into[i] }
}

// windowBits is every slot k's window follows, both kinds: what an operand of unknown origin may lend.
func (c *borrowCheck) windowBits(k *keptCtx) uint64 {
	n := len(c.declaredParams(k.g.Ent))
	if c.p.R.Entity(k.g.Ent).Kind == sem.EntClosure {
		n += len(c.hiddenLocals(k.g.Ent))
	}
	n = min(max(n-k.refs.base, 0), contentShift)
	own := uint64(1)<<n - 1
	return own | own<<contentShift
}

// refCallees are the callees call in may run: a resolved callable by its summaries (plus declaredRefKeeps through a value),
// a requirement through an interface value by every implementation, any other callable by valueKeeps on its type.
func (c *borrowCheck) refCallees(k *keptCtx, in midir.Inst) []calleeUse {
	switch {
	case (in.Op == midir.OpCallValue || in.Op == midir.OpFutureNew) && len(in.Args) > 0:
		if fns := c.refItems(c.keptView(k), in.Args[0], 8); fns != nil {
			return c.boundKeeps(k, fns, in.Args, 1)
		}
		if in.Ent != 0 && c.ifaceRequirement(in.Ent) && c.p.ByEnt[in.Ent] == nil {
			if uses := c.implementerKeeps(k, in.Ent, in.Args, 1); uses != nil {
				return uses
			}
		}
		s := c.valueKeeps(k.g.Locals[in.Args[0]].Type)
		return []calleeUse{{s: s, slots: argSlots(len(in.Args)-1, 1)}}
	case in.Op == midir.OpCall && in.Ent != 0 && c.ifaceRequirement(in.Ent):
		if uses := c.implementerKeeps(k, in.Ent, in.Args, 0); uses != nil {
			return uses
		}
		return []calleeUse{{s: &refSummary{all: true}, slots: argSlots(len(in.Args), 0)}}
	case in.Op == midir.OpCall && in.Ent != 0:
		n := min(len(c.declaredParams(in.Ent)), len(in.Args))
		return []calleeUse{{s: c.calleeRefKeeps(in.Ent, c.refBindIn(k, in.Ent, in.Args)), slots: argSlots(n, 0)}}
	}
	return nil
}

func argSlots(n, off int) []useSlot {
	var out []useSlot
	for j := range n {
		out = append(out, useSlot{j: j, arg: off + j})
	}
	return out
}

// implementerKeeps is boundKeeps over every implementation sem sealed for requirement req; nil when there is none.
func (c *borrowCheck) implementerKeeps(k *keptCtx, req sem.EntityID, args []midir.LocalID, off int) []calleeUse {
	var fns []boundFn
	for _, w := range c.implementers(req) {
		fns = append(fns, boundFn{ent: w})
	}
	if fns == nil {
		return nil
	}
	return c.boundKeeps(k, fns, args, off)
}

// boundKeeps are the uses of callables args[off-1] may hold, called with args[off:]: a fn item's summary joined with
// declaredRefKeeps, a method value's receiver and a closure's captures taken from the value itself.
func (c *borrowCheck) boundKeeps(k *keptCtx, fns []boundFn, args []midir.LocalID, off int) []calleeUse {
	var out []calleeUse
	for _, f := range fns {
		kind := c.p.R.Entity(f.ent).Kind
		if kind != sem.EntFn && kind != sem.EntClosure {
			return []calleeUse{{s: &refSummary{all: true}, slots: argSlots(len(args)-off, off)}}
		}
		n := len(c.declaredParams(f.ent))
		var slots []useSlot
		for j := range n {
			switch {
			case j < f.lead && off > 0:
				slots = append(slots, useSlot{j: j, arg: 0, via: true, applied: true})
			case off+j-f.lead < len(args):
				slots = append(slots, useSlot{j: j, arg: off + j - f.lead})
			}
		}
		if kind == sem.EntClosure && off > 0 {
			for i := range c.hiddenLocals(f.ent) {
				slots = append(slots, useSlot{j: n + i, arg: 0, via: true})
			}
		}
		lead := f.lead
		arg := func(j int) (midir.LocalID, bool) {
			if i := off + j - lead; j >= lead && i < len(args) {
				return args[i], true
			}
			return 0, false
		}
		s := c.calleeRefKeeps(f.ent, f.sub.with(c.refBind(c.keptView(k), f.ent, arg, func(a midir.LocalID) sem.TypeID { return k.g.Locals[a].Type })))
		if kind == sem.EntFn {
			s = s.join(c.declaredRefKeeps(f.ent))
		}
		out = append(out, calleeUse{s: s, slots: slots})
	}
	return out
}
