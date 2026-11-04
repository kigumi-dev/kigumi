package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// refOrigin is what a local of a refs run lends in window bits: own, the lent place it is or points into, held, the
// borrows its value or referent carries; unknown when some definition on the way is not followed.
type refOrigin struct {
	own, held uint64
	unknown   bool
}

func (o refOrigin) join(p refOrigin) refOrigin {
	return refOrigin{own: o.own | p.own, held: o.held | p.held, unknown: o.unknown || p.unknown}
}

// both is what a value keeps of o when it may hold o's borrow itself as well as what that borrow reaches.
func (o refOrigin) both() refOrigin {
	return refOrigin{own: o.own | o.held, held: o.own | o.held, unknown: o.unknown}
}

// refOrigins solves refOrigin for every local of k's body, flow-insensitively, to a fixpoint.
func (c *borrowCheck) refOrigins(k *keptCtx) map[midir.LocalID]refOrigin {
	if k.orig != nil {
		return k.orig
	}
	out := map[midir.LocalID]refOrigin{}
	for _, l := range k.g.Params {
		out[l] = c.paramOrigin(k, l)
	}
	for changed := true; changed; {
		changed = false
		for _, blk := range k.g.Blocks {
			for _, in := range blk.Insts {
				if !in.Op.Yields() || !definesDst(in) {
					continue
				}
				if o := out[in.Dst].join(c.defOrigin(k, in, out)); o != out[in.Dst] {
					out[in.Dst], changed = o, true
				}
			}
		}
	}
	k.orig = out
	return out
}

// paramOrigin: a parameter lends its own borrow when it is a reference, a cell or a borrowed receiver, and always what
// it holds; a fn's witness lends nothing, and a slot outside k's window is another run's.
func (c *borrowCheck) paramOrigin(k *keptCtx, l midir.LocalID) refOrigin {
	slot := k.slot(l)
	if slot < 0 || c.p.R.Entity(k.g.Ent).Kind == sem.EntFn && slot >= len(c.declaredParams(k.g.Ent)) {
		return refOrigin{}
	}
	i := slot - k.refs.base
	if i < 0 || i >= contentShift {
		return refOrigin{}
	}
	o := refOrigin{held: 1 << (i + contentShift)}
	if c.refLike(k.g, l) || k.g.Locals[l].Borrowed {
		o.own = 1 << i
	}
	return o
}

func (c *borrowCheck) refLike(g *midir.Func, l midir.LocalID) bool {
	return c.p.R.Types.Kind(g.Locals[l].Type) == sem.KRef || g.Locals[l].Cell
}

func (c *borrowCheck) scalarType(t sem.TypeID) bool {
	return c.p.R.Types.IsNumeric(t) || t == sem.TyBool || t == sem.TyChar || t == sem.TyUnit
}

func (c *borrowCheck) defOrigin(k *keptCtx, in midir.Inst, cur map[midir.LocalID]refOrigin) refOrigin {
	g := k.g
	if c.scalarType(g.Locals[in.Dst].Type) {
		return refOrigin{}
	}
	ref := c.refLike(g, in.Dst)
	arg := func() refOrigin { return cur[in.Args[0]] }
	switch {
	case bodyRoles[in.Op] == roleFresh || in.Op == midir.OpFnItem || in.Op == midir.OpCFnPtr:
		return refOrigin{}
	case bodyRoles[in.Op] == roleCall:
		var out refOrigin
		for _, a := range c.originOperands(g, in) {
			out = out.join(cur[a].both())
		}
		return out
	case len(in.Args) == 0:
		return refOrigin{unknown: true}
	}
	switch in.Op {
	case midir.OpCopy, midir.OpMove, midir.OpCellTake, midir.OpFieldMove:
		if ref {
			return arg()
		}
		return refOrigin{held: arg().held, unknown: arg().unknown}
	case midir.OpAlias, midir.OpShare, midir.OpNewCell, midir.OpBorrow:
		return arg()
	case midir.OpField, midir.OpIndex, midir.OpPayload, midir.OpUnbox, midir.OpUnboxDirect, midir.OpCellGet:
		if ref {
			return refOrigin{own: arg().held, held: arg().held, unknown: arg().unknown}
		}
		return arg()
	case midir.OpRecord, midir.OpVariant, midir.OpArray, midir.OpClosure, midir.OpBox, midir.OpBoxDirect, midir.OpBind, midir.OpLendBox:
		var out refOrigin
		for _, a := range in.Args {
			o := cur[a]
			if c.refLike(g, a) {
				o = o.both()
			}
			out = out.join(refOrigin{held: o.held, unknown: o.unknown})
		}
		return out
	}
	return refOrigin{unknown: true}
}

// originOperands are the operands a call's result may borrow from: the non-scalar ones, only those the callee's
// declared result lifetime names when it declares one.
func (c *borrowCheck) originOperands(g *midir.Func, in midir.Inst) []midir.LocalID {
	var all []midir.LocalID
	for _, a := range in.Args {
		if !c.scalarType(g.Locals[a].Type) {
			all = append(all, a)
		}
	}
	if in.Op != midir.OpCall || in.Ent == 0 || c.p.R.Entity(in.Ent).Kind != sem.EntFn {
		return all
	}
	info := c.p.R.Fn(in.Ent)
	lt := c.fnRetLifetime(info)
	if lt == 0 {
		return all
	}
	var out []midir.LocalID
	for j, q := range c.declaredParams(in.Ent) {
		if j >= len(in.Args) {
			break
		}
		named := q == info.SelfParam && (c.p.R.Local(q).Lifetime == lt || slices.Contains(c.recvLifetimeParams(info), lt))
		if named || q != info.SelfParam && c.paramLifetimeOf(info, q) == lt {
			out = append(out, in.Args[j])
		}
	}
	if out == nil {
		return all
	}
	return out
}
