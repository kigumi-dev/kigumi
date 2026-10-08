package mir

import (
	"strings"

	"kigumi/internal/sem"
)

type primShape uint8

const (
	shapeNone primShape = iota
	shapeScalar
	shapeOption
)

func (p *Program) markDirectPrims(f *Func) {
	for bid := range f.Blocks {
		insts := f.Blocks[bid].Insts
		for i := range insts {
			if insts[i].Op == OpCall {
				insts[i].DPrim = p.directPrimOf(f, insts[i])
			}
		}
	}
}

func (p *Program) directPrimOf(f *Func, in Inst) DirectPrim {
	if _, own := p.ByEnt[in.Ent]; own || !p.IsStdPrimitive(in.Ent) {
		return DPNone
	}
	info := p.R.Fn(in.Ent)
	if info.Naked || p.R.Entity(p.R.Entity(in.Ent).Parent).Kind == sem.EntInterface {
		return DPNone
	}
	kind, ok := DPNone, false
	if id, found := in.StdOp(); found && id.ResultABI() == ResultDirect {
		kind, ok = id.directPrim(), true
	} else if p.isConversion(in.Ent) {
		kind, ok = DPConvert, true
	}
	if !ok || !p.primTypesFit(f, in, kind) {
		return DPNone
	}
	return kind
}

func (p *Program) resultShape(t sem.TypeID) primShape {
	if p.R.RegScalar(t) {
		return shapeScalar
	}
	if elem, ok := p.R.Types.IsOption(t); ok && p.R.RegScalar(elem) && p.R.DirectAgg(t) {
		return shapeOption
	}
	return shapeNone
}

func (p *Program) primTypesFit(f *Func, in Inst, kind DirectPrim) bool {
	arg := func(i int) sem.TypeID { return f.Locals[in.Args[i]].Type }
	shape := p.resultShape(f.Locals[in.Dst].Type)
	switch kind {
	case DPLen, DPArrayLen:
		return len(in.Args) == 1 && shape == shapeScalar
	case DPBytesGet, DPArrayGet:
		return len(in.Args) == 2 && p.R.RegScalar(arg(1)) && shape == shapeOption
	case DPCharFromInt:
		return len(in.Args) == 1 && p.R.RegScalar(arg(0)) && shape == shapeOption && p.optionElem(f.Locals[in.Dst].Type) == sem.TyChar
	case DPParseFloat:
		return len(in.Args) == 1 && shape == shapeOption && p.optionElem(f.Locals[in.Dst].Type) == sem.TyF64
	case DPConvert:
		return len(in.Args) == 1 && p.numericOrChar(arg(0)) && shape != shapeNone && p.numericOrChar(p.leafOf(f.Locals[in.Dst].Type))
	case DPSqrt, DPFloor, DPCeil, DPRound:
		return len(in.Args) == 1 && arg(0) == sem.TyF64 && f.Locals[in.Dst].Type == sem.TyF64
	case DPPow:
		return len(in.Args) == 2 && arg(0) == sem.TyF64 && arg(1) == sem.TyF64 && f.Locals[in.Dst].Type == sem.TyF64
	}
	if kind >= DPWrapAdd && kind <= DPRotr {
		t := f.Locals[in.Dst].Type
		return len(in.Args) == 2 && p.R.RegScalar(t) && p.R.Types.IsInteger(t) && arg(0) == t && arg(1) == t
	}
	return false
}

func (p *Program) optionElem(t sem.TypeID) sem.TypeID {
	elem, _ := p.R.Types.IsOption(t)
	return elem
}

func (p *Program) leafOf(t sem.TypeID) sem.TypeID {
	if elem, ok := p.R.Types.IsOption(t); ok {
		return elem
	}
	return t
}

func (p *Program) numericOrChar(t sem.TypeID) bool {
	return p.R.RegScalar(t) && (t == sem.TyChar || p.R.Types.IsNumeric(t))
}

func (p *Program) isConversion(fn sem.EntityID) bool {
	info := p.R.Fn(fn)
	if info.Owner == 0 || !strings.HasPrefix(p.R.Entity(fn).Name, "to") || !strings.HasPrefix(StdKey(p.R, fn, false), "std/prelude.") {
		return false
	}
	self := p.R.Entity(info.Owner).Type
	return p.R.Types.IsNumeric(self) || self == sem.TyChar
}

func (p *Program) markPrimResult(f *Func, in Inst, out []bool) {
	if in.Op == OpCall && in.DPrim != DPNone && p.R.DirectAgg(f.Locals[in.Dst].Type) {
		out[in.Dst] = true
	}
}

func (p *Program) primResult(f *Func, in Inst) sem.TypeID {
	if t := f.Locals[in.Dst].Type; in.DPrim != DPNone && p.R.DirectAgg(t) {
		return t
	}
	return 0
}
