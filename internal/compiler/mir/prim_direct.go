package mir

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

type primShape uint8

const (
	shapeNone primShape = iota
	shapeScalar
	shapeOption
)

func (p *Program) DirectPrimOf(f *midir.Func, in midir.Inst) opdef.DirectPrim {
	if _, own := p.ByEnt[in.Ent]; own || !ownquery.IsStdPrimitive(p.R, in.Ent) {
		return opdef.DPNone
	}
	info := p.R.Fn(in.Ent)
	if info.Naked || p.R.Entity(p.R.Entity(in.Ent).Parent).Kind == sem.EntInterface {
		return opdef.DPNone
	}
	kind, ok := opdef.DPNone, false
	if id, found := in.StdOp(); found && id.ResultABI() == opdef.ResultDirect {
		kind, ok = id.DirectPrimitive(), true
	} else if p.isConversion(in.Ent) {
		kind, ok = opdef.DPConvert, true
	}
	if !ok || !p.PrimTypesFit(f, in, kind) {
		return opdef.DPNone
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

func (p *Program) PrimTypesFit(f *midir.Func, in midir.Inst, kind opdef.DirectPrim) bool {
	arg := func(i int) sem.TypeID { return f.Locals[in.Args[i]].Type }
	shape := p.resultShape(f.Locals[in.Dst].Type)
	switch kind {
	case opdef.DPLen, opdef.DPArrayLen:
		return len(in.Args) == 1 && shape == shapeScalar
	case opdef.DPMapSeed:
		return len(in.Args) == 0 && shape == shapeScalar
	case opdef.DPBytesGet, opdef.DPArrayGet:
		return len(in.Args) == 2 && p.R.RegScalar(arg(1)) && shape == shapeOption
	case opdef.DPArrayFirst, opdef.DPArrayLast:
		return len(in.Args) == 1 && shape == shapeOption
	case opdef.DPCharFromInt:
		return len(in.Args) == 1 && p.R.RegScalar(arg(0)) && shape == shapeOption && p.optionElem(f.Locals[in.Dst].Type) == sem.TyChar
	case opdef.DPParseFloat:
		return len(in.Args) == 1 && shape == shapeOption && p.optionElem(f.Locals[in.Dst].Type) == sem.TyF64
	case opdef.DPConvert:
		return len(in.Args) == 1 && p.numericOrChar(arg(0)) && shape != shapeNone && p.numericOrChar(p.leafOf(f.Locals[in.Dst].Type))
	case opdef.DPSqrt, opdef.DPFloor, opdef.DPCeil, opdef.DPRound:
		return len(in.Args) == 1 && arg(0) == sem.TyF64 && f.Locals[in.Dst].Type == sem.TyF64
	case opdef.DPToBits:
		return len(in.Args) == 1 && arg(0) == sem.TyF64 && f.Locals[in.Dst].Type == sem.TyU64
	case opdef.DPFromBits:
		return len(in.Args) == 1 && arg(0) == sem.TyU64 && f.Locals[in.Dst].Type == sem.TyF64
	case opdef.DPSecretBarrier, opdef.DPSecretMaskNonZero:
		return len(in.Args) == 1 && p.R.RegScalar(arg(0)) && shape == shapeScalar
	case opdef.DPSecretMaskLt:
		return len(in.Args) == 2 && p.R.RegScalar(arg(0)) && p.R.RegScalar(arg(1)) && shape == shapeScalar
	case opdef.DPSecretSelect:
		return len(in.Args) == 3 && p.R.RegScalar(arg(0)) && p.R.RegScalar(arg(1)) && p.R.RegScalar(arg(2)) && shape == shapeScalar
	}
	if kind == opdef.DPTryRef || kind == opdef.DPTryOpt {
		return p.tryRefResult(f.Locals[in.Dst].Type, kind == opdef.DPTryOpt)
	}
	if kind >= opdef.DPWrapAdd && kind <= opdef.DPRotr {
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
	return p.R.RegScalar(t) && t != sem.TyF32 && (t == sem.TyChar || p.R.Types.IsNumeric(t))
}

func (p *Program) isConversion(fn sem.EntityID) bool {
	info := p.R.Fn(fn)
	if info.Owner == 0 || !p.R.IsConversion(fn) || !strings.HasPrefix(opdef.StdKey(p.R, fn, false), "std/prelude.") {
		return false
	}
	self := p.R.Entity(info.Owner).Type
	return p.R.Types.IsNumeric(self) || self == sem.TyChar
}

func (p *Program) primResult(f *midir.Func, in midir.Inst) sem.TypeID {
	if t := f.Locals[in.Dst].Type; in.DPrim != opdef.DPNone && p.R.DirectAgg(t) {
		return t
	}
	return 0
}
