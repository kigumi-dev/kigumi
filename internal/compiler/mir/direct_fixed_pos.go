package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

func (p *Program) FixedBase(f *midir.Func, base midir.LocalID) sem.TypeID {
	t := p.WholePos(f, base)
	if t == 0 || !p.R.DirectFixed(t) {
		return 0
	}
	return t
}

func (p *Program) elemPos(f *midir.Func, base midir.LocalID) sem.TypeID {
	t := p.FixedBase(f, base)
	if t == 0 {
		return 0
	}
	elem, _, _ := p.R.FixedElem(t)
	if !p.R.DirectAgg(elem) {
		return 0
	}
	return elem
}

func IterBuiltin(in midir.Inst) bool {
	s, ok := opdef.BuiltinFor(in.Str)
	return in.Op == midir.OpBuiltin && ok && s.Iter
}

func FixedFill(in midir.Inst) bool { return in.Op == midir.OpBuiltin && in.Str == "fixed.fill" }

func (p *Program) fixedArgPos(f *midir.Func, in midir.Inst, i int) sem.TypeID {
	switch {
	case ProtoBuiltin(in):
		if f.DirectPlanned() {
			return f.Locals[in.Args[i]].Type
		}
	case in.Op == midir.OpIndex || IterBuiltin(in):
		if i == 0 {
			return p.FixedBase(f, in.Args[0])
		}
	case in.Op == midir.OpSetIndex:
		switch i {
		case 0:
			return p.FixedBase(f, in.Args[0])
		case 2:
			return p.elemPos(f, in.Args[0])
		}
	case FixedFill(in):
		elem, _, _ := p.R.FixedElem(f.Locals[in.Dst].Type)
		if i == 0 && p.WholePos(f, in.Dst) != 0 && p.R.DirectAgg(elem) {
			return elem
		}
	}
	return 0
}

func (p *Program) fixedDstPos(f *midir.Func, in midir.Inst) sem.TypeID {
	switch {
	case FixedFill(in):
		return p.WholePos(f, in.Dst)
	case in.Op == midir.OpIndex || (IterBuiltin(in) && in.Str != "iter.len"):
		return p.elemPos(f, in.Args[0])
	}
	return 0
}
