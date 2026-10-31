package mir

import (
	"fmt"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// A crossing is produced and consumed between two neighbouring
// instructions, so it never needs a frame slot of its own.
func crossTemp(f *midir.Func, t sem.TypeID, direct bool) midir.LocalID {
	f.Locals = append(f.Locals, midir.Local{Name: fmt.Sprintf("x%d", len(f.Locals)), Type: t, Cross: true})
	f.Direct = append(f.Direct, direct)
	if len(f.Slot) > 0 {
		f.Slot = append(f.Slot, -1)
	}
	return midir.LocalID(len(f.Locals) - 1)
}

// replaceArg copies on first write: the builder shares argument slices.
func replaceArg(args []midir.LocalID, i int, l midir.LocalID) []midir.LocalID {
	cp := make([]midir.LocalID, len(args))
	copy(cp, args)
	cp[i] = l
	return cp
}

// argPos names the type the position expects, 0 for a carrier; a local's
// own classification doesn't decide this, the position does.
func (p *Program) argPos(f *midir.Func, in midir.Inst, i int) sem.TypeID {
	switch in.Op {
	case midir.OpCall:
		fn, ok := p.ByEnt[in.Ent]
		if !ok {
			return 0
		}
		return fn.DSig.Param(i)
	case midir.OpRecord:
		return p.fieldPos(f, in.Dst, i)
	case midir.OpClosure:
		return p.envPos(f, in.Dst, i)
	case midir.OpCallValue:
		if i == 0 {
			return p.wholePos(f, in.Args[0])
		}
		return p.valueArgPos(f, in, i)
	case midir.OpVariant:
		return p.payloadPos(f, in.Dst, in.Ent, i)
	case midir.OpPayload, midir.OpIsVariant:
		return p.wholePos(f, in.Args[0])
	case midir.OpSetField:
		if i == 1 {
			return p.fieldPos(f, in.Args[0], in.Index)
		}
		return p.wholePos(f, in.Args[0])
	case midir.OpField, midir.OpFieldMove, midir.OpDrop, midir.OpRelease, midir.OpFutureDone:
		return p.wholePos(f, in.Args[0])
	case midir.OpAlias, midir.OpCopy, midir.OpShare:
		return p.wholePos(f, in.Dst)
	case midir.OpMove:
		return p.moveCrossPos(f, in)
	case midir.OpBoxDirect:
		return in.Type
	case midir.OpIndex, midir.OpSetIndex, midir.OpBuiltin:
		return p.fixedArgPos(f, in, i)
	}
	return 0
}

func (p *Program) dstPos(f *midir.Func, in midir.Inst) sem.TypeID {
	switch in.Op {
	case midir.OpCall:
		fn, ok := p.ByEnt[in.Ent]
		if !ok {
			return p.primResult(f, in)
		}
		return fn.DSig.Result()
	case midir.OpField, midir.OpFieldMove:
		return p.fieldPos(f, in.Args[0], in.Index)
	case midir.OpPayload:
		return p.payloadDst(f, in)
	case midir.OpRecord, midir.OpVariant, midir.OpClosure, midir.OpAlias, midir.OpCopy, midir.OpShare:
		return p.wholePos(f, in.Dst)
	case midir.OpMove:
		return p.moveCrossPos(f, in)
	case midir.OpUnboxDirect:
		return in.Type
	case midir.OpIndex, midir.OpBuiltin:
		return p.fixedDstPos(f, in)
	case midir.OpCallValue:
		if body := p.valueCallee(f, in); body != nil {
			return body.DSig.Result()
		}
	}
	return 0
}

// A move out of a named local keeps the local's form so the move still empties it; the result crosses afterwards.
func (p *Program) moveCrossPos(f *midir.Func, in midir.Inst) sem.TypeID {
	if f.Locals[in.Args[0]].Ent != 0 {
		return p.wholePos(f, in.Args[0])
	}
	return p.wholePos(f, in.Dst)
}

// wholePos reads the aggregate out of the local that names it.
func (p *Program) wholePos(f *midir.Func, l midir.LocalID) sem.TypeID {
	if !f.IsDirect(l) {
		return 0
	}
	return f.Locals[l].Type
}

func (p *Program) fieldPos(f *midir.Func, base midir.LocalID, i int) sem.TypeID {
	t := p.wholePos(f, base)
	if t == 0 || p.R.DirectAdt(t) {
		return 0
	}
	return p.leafPos(t, p.R.AggFields(t), i)
}

func (p *Program) envPos(f *midir.Func, clo midir.LocalID, i int) sem.TypeID {
	t := p.wholePos(f, clo)
	if t == 0 || !p.R.DirectClosure(t) {
		return 0
	}
	return p.leafPos(t, p.R.AggFields(t), i)
}

func (p *Program) payloadPos(f *midir.Func, base midir.LocalID, v sem.EntityID, i int) sem.TypeID {
	t := p.wholePos(f, base)
	if t == 0 || !p.R.DirectAdt(t) {
		return 0
	}
	return p.leafPos(t, p.R.AggPayload(t, v), i)
}

func (p *Program) leafPos(t sem.TypeID, leaves []sem.TypeID, i int) sem.TypeID {
	if i < 0 || i >= len(leaves) || !p.R.DirectAgg(leaves[i]) {
		return 0
	}
	return leaves[i]
}

// A payload read names no variant, so the binding's own type says what comes out of the slot.
func (p *Program) payloadDst(f *midir.Func, in midir.Inst) sem.TypeID {
	if p.wholePos(f, in.Args[0]) == 0 {
		return 0
	}
	t := f.Locals[in.Dst].Type
	if !p.R.DirectAgg(t) {
		return 0
	}
	return t
}

// countlessRefs marks the reference temporaries holding no count of their own,
// so a crossing out of one gives nothing back when its consumer keeps the copy.
func (p *Program) countlessRefs(f *midir.Func) []bool {
	c := refCounts{defs: map[midir.LocalID][]midir.Inst{}, owns: map[midir.LocalID]bool{}}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() && f.Locals[in.Dst].Ent == 0 {
				c.defs[in.Dst] = append(c.defs[in.Dst], in)
			}
		}
	}
	out := make([]bool, len(f.Locals))
	for l, loc := range f.Locals {
		out[l] = loc.Ent == 0 && p.R.Types.Kind(loc.Type) == sem.KRef && !c.own(midir.LocalID(l))
	}
	return out
}

type refCounts struct {
	defs map[midir.LocalID][]midir.Inst
	owns map[midir.LocalID]bool
}

// own follows aliases back to where the value was made: a call, a field read or a share takes a count,
// while a borrow, a lent element and an alias of a named local take none.
func (c *refCounts) own(l midir.LocalID) bool {
	if v, ok := c.owns[l]; ok {
		return v
	}
	c.owns[l] = false
	v := len(c.defs[l]) > 0
	for _, in := range c.defs[l] {
		switch {
		case in.Op == midir.OpBorrow, in.Op == midir.OpLendBox, in.Op == midir.OpBuiltin && in.Str == "iter.at_ref":
			v = false
		case (in.Op == midir.OpAlias || in.Op == midir.OpMove || in.Op == midir.OpCopy && !in.Share) && len(in.Args) == 1:
			v = v && c.own(in.Args[0])
		}
	}
	c.owns[l] = v
	return v
}
