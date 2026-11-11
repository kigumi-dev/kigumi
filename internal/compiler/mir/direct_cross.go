package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// ArgPos names the type the position expects, 0 for a carrier; a local's
// own classification doesn't decide this, the position does.
func (p *Program) ArgPos(f *midir.Func, in midir.Inst, i int) sem.TypeID {
	switch in.Op {
	case midir.OpCall:
		if p.SimdCall(in) && f.DirectPlanned() {
			if i < len(in.Args) && p.R.DirectAgg(f.Locals[in.Args[i]].Type) {
				return f.Locals[in.Args[i]].Type
			}
			return 0
		}
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
			return p.WholePos(f, in.Args[0])
		}
		return p.valueArgPos(f, in, i)
	case midir.OpVariant:
		return p.payloadPos(f, in.Dst, in.Ent, i)
	case midir.OpPayload, midir.OpIsVariant:
		return p.WholePos(f, in.Args[0])
	case midir.OpSetField:
		if i == 1 {
			return p.fieldPos(f, in.Args[0], in.Index)
		}
		return p.WholePos(f, in.Args[0])
	case midir.OpField, midir.OpFieldMove, midir.OpDrop, midir.OpRelease, midir.OpFutureDone:
		return p.WholePos(f, in.Args[0])
	case midir.OpAlias, midir.OpCopy, midir.OpShare:
		return p.WholePos(f, in.Dst)
	case midir.OpMove:
		return p.moveCrossPos(f, in)
	case midir.OpBoxDirect:
		return in.Type
	case midir.OpIndex, midir.OpSetIndex, midir.OpBuiltin:
		return p.fixedArgPos(f, in, i)
	}
	return 0
}

func (p *Program) DstPos(f *midir.Func, in midir.Inst) sem.TypeID {
	switch in.Op {
	case midir.OpCall:
		if p.SimdCall(in) && f.DirectPlanned() {
			if int(in.Dst) < len(f.Locals) && p.R.DirectAgg(f.Locals[in.Dst].Type) {
				return f.Locals[in.Dst].Type
			}
			return 0
		}
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
		return p.WholePos(f, in.Dst)
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
		return p.WholePos(f, in.Args[0])
	}
	return p.WholePos(f, in.Dst)
}

// WholePos reads the aggregate out of the local that names it.
func (p *Program) WholePos(f *midir.Func, l midir.LocalID) sem.TypeID {
	if !f.IsDirect(l) {
		return 0
	}
	return f.Locals[l].Type
}

func (p *Program) fieldPos(f *midir.Func, base midir.LocalID, i int) sem.TypeID {
	t := p.WholePos(f, base)
	if t == 0 || p.R.DirectAdt(t) {
		return 0
	}
	return p.leafPos(t, p.R.AggFields(t), i)
}

func (p *Program) envPos(f *midir.Func, clo midir.LocalID, i int) sem.TypeID {
	t := p.WholePos(f, clo)
	if t == 0 || !p.R.DirectClosure(t) {
		return 0
	}
	return p.leafPos(t, p.R.AggFields(t), i)
}

func (p *Program) payloadPos(f *midir.Func, base midir.LocalID, v sem.EntityID, i int) sem.TypeID {
	t := p.WholePos(f, base)
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
	if p.WholePos(f, in.Args[0]) == 0 {
		return 0
	}
	t := f.Locals[in.Dst].Type
	if !p.R.DirectAgg(t) {
		return 0
	}
	return t
}
