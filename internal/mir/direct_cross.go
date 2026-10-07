package mir

import (
	"fmt"

	"kigumi/internal/sem"
)

// A crossing is produced and consumed between two neighbouring
// instructions, so it never needs a frame slot of its own.
func (f *Func) temp(t sem.TypeID, direct bool) LocalID {
	f.Locals = append(f.Locals, Local{Name: fmt.Sprintf("x%d", len(f.Locals)), Type: t, Cross: true})
	f.Direct = append(f.Direct, direct)
	if len(f.Slot) > 0 {
		f.Slot = append(f.Slot, -1)
	}
	return LocalID(len(f.Locals) - 1)
}

// replaceArg copies on first write: the builder shares argument slices.
func replaceArg(args []LocalID, i int, l LocalID) []LocalID {
	cp := make([]LocalID, len(args))
	copy(cp, args)
	cp[i] = l
	return cp
}

// argPos names the type the position expects, 0 for a carrier; a local's
// own classification doesn't decide this, the position does.
func (p *Program) argPos(f *Func, in Inst, i int) sem.TypeID {
	switch in.Op {
	case OpCall:
		fn, ok := p.ByEnt[in.Ent]
		if !ok {
			return 0
		}
		return fn.DSig.Param(i)
	case OpRecord:
		return p.fieldPos(f, in.Dst, i)
	case OpSetField:
		if i == 1 {
			return p.fieldPos(f, in.Args[0], in.Index)
		}
		return p.wholePos(f, in.Args[0])
	case OpField, OpFieldMove, OpDrop, OpRelease:
		return p.wholePos(f, in.Args[0])
	case OpAlias, OpMove, OpCopy, OpShare:
		return p.wholePos(f, in.Dst)
	case OpBoxDirect:
		return in.Type
	}
	return 0
}

func (p *Program) dstPos(f *Func, in Inst) sem.TypeID {
	switch in.Op {
	case OpCall:
		fn, ok := p.ByEnt[in.Ent]
		if !ok {
			return 0
		}
		return fn.DSig.result()
	case OpField, OpFieldMove:
		return p.fieldPos(f, in.Args[0], in.Index)
	case OpRecord, OpAlias, OpMove, OpCopy, OpShare:
		return p.wholePos(f, in.Dst)
	case OpUnboxDirect:
		return in.Type
	}
	return 0
}

// wholePos reads the aggregate out of the local that names it.
func (p *Program) wholePos(f *Func, l LocalID) sem.TypeID {
	if !f.IsDirect(l) {
		return 0
	}
	return f.Locals[l].Type
}

func (p *Program) fieldPos(f *Func, base LocalID, i int) sem.TypeID {
	t := p.wholePos(f, base)
	if t == 0 {
		return 0
	}
	fields := p.R.AggFields(t)
	if i < 0 || i >= len(fields) || !p.R.DirectAgg(fields[i]) {
		return 0
	}
	return fields[i]
}
