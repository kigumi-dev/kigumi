package mir

import "kigumi/internal/sem"

func (p *Program) fixedBase(f *Func, base LocalID) sem.TypeID {
	t := p.wholePos(f, base)
	if t == 0 || !p.R.DirectFixed(t) {
		return 0
	}
	return t
}

func (p *Program) elemPos(f *Func, base LocalID) sem.TypeID {
	t := p.fixedBase(f, base)
	if t == 0 {
		return 0
	}
	elem, _, _ := p.R.FixedElem(t)
	if !p.R.DirectAgg(elem) {
		return 0
	}
	return elem
}

func iterBuiltin(in Inst) bool {
	if in.Op != OpBuiltin {
		return false
	}
	switch in.Str {
	case "iter.len", "iter.at", "iter.at_ref", "iter.at_move":
		return true
	}
	return false
}

func fixedFill(in Inst) bool { return in.Op == OpBuiltin && in.Str == "fixed.fill" }

func (p *Program) fixedArgPos(f *Func, in Inst, i int) sem.TypeID {
	switch {
	case in.Op == OpIndex || iterBuiltin(in):
		if i == 0 {
			return p.fixedBase(f, in.Args[0])
		}
	case in.Op == OpSetIndex:
		switch i {
		case 0:
			return p.fixedBase(f, in.Args[0])
		case 2:
			return p.elemPos(f, in.Args[0])
		}
	case fixedFill(in):
		elem, _, _ := p.R.FixedElem(f.Locals[in.Dst].Type)
		if i == 0 && p.wholePos(f, in.Dst) != 0 && p.R.DirectAgg(elem) {
			return elem
		}
	}
	return 0
}

func (p *Program) fixedDstPos(f *Func, in Inst) sem.TypeID {
	switch {
	case fixedFill(in):
		return p.wholePos(f, in.Dst)
	case in.Op == OpIndex || (iterBuiltin(in) && in.Str != "iter.len"):
		return p.elemPos(f, in.Args[0])
	}
	return 0
}

func (p *Program) fixedUse(f *Func, in Inst, out []bool) {
	switch {
	case fixedFill(in):
		out[in.Dst] = true
	case in.Op == OpIndex || in.Op == OpSetIndex || iterBuiltin(in):
		if p.R.IsFixedArray(f.Locals[in.Args[0]].Type) {
			out[in.Args[0]] = true
		}
	}
}
