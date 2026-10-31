package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) collectValue(in midir.Inst, bid midir.BlockID, idx int) {
	c.collectCarry(in, bid, idx)
	c.collectFnCaps(in)
}

// collectFnCaps follows a closure's captured borrows along every value that may hold it, so the tie sees them whatever the spelling.
func (c *borrowCheck) collectFnCaps(in midir.Inst) {
	dst, srcs := in.Dst, in.Args
	switch {
	case in.Op == midir.OpClosure && c.p.R.Entity(in.Ent).Kind == sem.EntClosure:
		if !c.resultMayBorrow(c.p.R.Closure(in.Ent).Ret) {
			return
		}
		c.addFnCaps(dst, c.returnedCaptures(&in), in)
	case in.Op == midir.OpSetField || in.Op == midir.OpSetIndex || in.Op == midir.OpCellSet:
		root, _, _, ok := c.projectionPlace(in.Args[0])
		if !ok {
			return
		}
		dst, srcs = root, srcs[len(srcs)-1:]
	case !in.Op.Yields() || freshValue[in.Op] || !c.mayHoldFn(c.f.Locals[dst].Type):
		return
	}
	for _, a := range srcs {
		c.addFnCaps(dst, c.ties.caps[a], in)
	}
}

// freshValue lists the opcodes whose result cannot carry a fn value from an operand; everything else unions its operands.
var freshValue = map[midir.Opcode]bool{midir.OpConst: true, midir.OpUnit: true, midir.OpBinary: true, midir.OpUnary: true, midir.OpTag: true, midir.OpIsVariant: true,
	midir.OpIsType: true, midir.OpInterp: true, midir.OpShell: true, midir.OpFnItem: true, midir.OpCFnPtr: true, midir.OpGlobalGet: true, midir.OpCancelled: true}

func (c *borrowCheck) mayHoldFn(t sem.TypeID) bool {
	return c.typeHolds(t, func(t sem.TypeID) bool {
		n := c.p.R.Types.Node(t)
		switch n.Kind {
		case sem.KFn, sem.KClosure, sem.KIface, sem.KPtr, sem.KVar:
			return true
		case sem.KParam:
			return !c.p.R.TypeParam(n.Ent).IsLifetime
		case sem.KNamed:
			_, opaque := c.namedParts(t)
			return opaque
		}
		return false
	})
}

func (c *borrowCheck) addFnCaps(dst midir.LocalID, b *borrowInfo, in midir.Inst) {
	if b == nil || b.empty {
		return
	}
	if c.ties.caps == nil {
		c.ties.caps = map[midir.LocalID]*borrowInfo{}
	}
	s, ok := c.ties.caps[dst]
	if !ok {
		s = &borrowInfo{empty: true, node: in.Node}
		c.ties.caps[dst] = s
	}
	c.merge(s, b)
}

func (c *borrowCheck) flowInfo(f flowArg) (*borrowInfo, bool) {
	b, ok := c.flowInfoOf(f)
	caps := c.ties.caps[f.arg]
	switch {
	case caps == nil || caps.empty:
		return b, ok
	case !ok:
		return caps, true
	}
	out := &borrowInfo{empty: true, node: b.node}
	mergeInto(out, b)
	mergeInto(out, caps)
	return out, true
}
