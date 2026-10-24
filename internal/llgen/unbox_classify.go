package llgen

import (
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// scalarInfo is a local materialized in a native register: Ty is the
// register type, Nk the numeric-kind tag rt_int and the inline integer ops expect
// (meaningless for TDouble, TFloat, TI1 and TI32).
type scalarInfo struct {
	Ty Type
	Nk int
}

// scalarType covers Bool, Char, f32 (an LLVM float), f64 and every integer width;
// a 128-bit integer is an i128 in a register and a two-word slot in a direct aggregate.
func (e *emitter) scalarType(t sem.TypeID) (Type, bool) {
	tt := e.r.Types
	switch {
	case t == sem.TyBool:
		return TI1, true
	case t == sem.TyChar:
		return TI32, true
	case t == sem.TyF64:
		return TDouble, true
	case t == sem.TyF32:
		return TFloat, true
	case t == sem.TyI128 || t == sem.TyU128:
		return TI128, true
	case tt.IsInteger(t):
		return TI64, true
	}
	return TVoid, false
}

func (e *emitter) scalarBoxed(t sem.TypeID) bool {
	tt := e.r.Types
	return t == sem.TyBool || t == sem.TyChar || tt.IsInteger(t) || tt.IsFloat(t)
}

func (e *emitter) classifyScalars(f *mir.Func) map[mir.LocalID]scalarInfo {
	out := map[mir.LocalID]scalarInfo{}
	isParam := make([]bool, len(f.Locals))
	inReg := make([]bool, len(f.Locals))
	ncaps := 0
	if env, ok := e.envOf(f); ok {
		ncaps = len(env.Fields)
	}
	sig := e.ssigs[f]
	for i, p := range f.Params {
		_, native := sig.param(i)
		isParam[p] = i >= ncaps && !native
		inReg[p] = i < ncaps || native
	}
	for i, l := range f.Locals {
		if isParam[i] || l.Cell || (l.Borrowed && !inReg[i]) {
			continue
		}
		if ty, ok := e.scalarType(l.Type); ok && (ty != TI128 || !framed(f, mir.LocalID(i))) {
			out[mir.LocalID(i)] = scalarInfo{Ty: ty, Nk: e.numKind(l.Type)}
		}
	}
	disqualify := func(l mir.LocalID) { delete(out, l) }
	e.raw = e.rawScalars(f)
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			e.classifyDst(f, in, disqualify)
			e.classifyArgs(f, in, disqualify)
			// A wrap/unwrap alias from a reference keeps its source's boxed
			// pointer bits, so its result can't live in a native register.
			if in.Op == mir.OpAlias && e.r.Types.Kind(f.Locals[in.Args[0]].Type) == sem.KRef {
				disqualify(in.Dst)
			}
		}
		classifyTerm(blk.Term, out, disqualify)
	}
	e.keepReturnedBoxes(f, disqualify)
	e.keepAliasedBoxes(f, out, disqualify)
	return out
}

// classifyDst disqualifies in.Dst unless in.Op is one the scalar codegen
// computes without ever creating a box for its result.
func (e *emitter) classifyDst(f *mir.Func, in mir.Inst, disqualify func(mir.LocalID)) {
	if !in.Op.Yields() {
		return
	}
	switch in.Op {
	case mir.OpField, mir.OpFieldMove, mir.OpPayload, mir.OpIsVariant, mir.OpIndex:
		if !e.isDirect(in.Args[0]) {
			disqualify(in.Dst)
		}
	case mir.OpBuiltin:
		if (!e.inPlaceBuiltin(in) || in.Str == "fixed.fill") && !e.iterRaw(f, in) {
			disqualify(in.Dst)
		}
	case mir.OpConst, mir.OpBinary, mir.OpAlias, mir.OpShare, mir.OpMove:
	case mir.OpGlobalGet:
		if _, ok := e.directStatic(in.Ent); !ok {
			disqualify(in.Dst)
		}
	case mir.OpUnary:
		if in.Str == "cast" {
			disqualify(in.Dst)
		}
	case mir.OpCall:
		if _, ok := e.calleeScalarSig(in).ret(); in.DPrim == mir.DPNone && !ok && e.wideCallOp(f, in) == "" && !e.rawConvert(f, in) {
			disqualify(in.Dst)
		}
	case mir.OpCallValue:
		if !e.callvI64(f, in) {
			disqualify(in.Dst)
		}
	default:
		disqualify(in.Dst)
	}
}

// classifyArgs disqualifies locals an instruction can't read natively.
// OpIndex/OpSetIndex allow the index arg boxed-on-demand; OpDrop/OpRelease
// become no-ops instead of disqualifying; everything else needs a real box.
func (e *emitter) classifyArgs(f *mir.Func, in mir.Inst, disqualify func(mir.LocalID)) {
	switch in.Op {
	case mir.OpRecord, mir.OpVariant, mir.OpClosure:
		if !e.isDirect(in.Dst) {
			for _, a := range in.Args {
				disqualify(a)
			}
		}
	case mir.OpSetField:
		if !e.isDirect(in.Args[0]) {
			for _, a := range in.Args {
				disqualify(a)
			}
		}
	case mir.OpBuiltin:
		if !e.inPlaceBuiltin(in) {
			for i, a := range in.Args {
				if i != 1 || iterModes[in.Str] == 0 {
					disqualify(a)
				}
			}
		}
	case mir.OpBorrow:
		disqualify(in.Args[0])
	case mir.OpAlias, mir.OpShare, mir.OpMove:
	case mir.OpUnary:
		if in.Str == "cast" {
			disqualify(in.Args[0])
		}
	case mir.OpBinary:
	case mir.OpInterp:
		for _, a := range in.Args {
			if !e.raw[a] {
				disqualify(a)
			}
		}
	case mir.OpIndex, mir.OpSetIndex:
		for i, a := range in.Args {
			if i != 1 && !(i == 2 && e.isDirect(in.Args[0])) {
				disqualify(a)
			}
		}
	case mir.OpDrop, mir.OpRelease:
	case mir.OpGlobalSet:
		if _, ok := e.directStatic(in.Ent); !ok {
			disqualify(in.Args[0])
		}
	case mir.OpCall:
		if in.DPrim == mir.DPNone && e.wideCallOp(f, in) == "" && !e.rawConvert(f, in) {
			sig := e.calleeScalarSig(in)
			for i, a := range in.Args {
				if _, ok := sig.param(i); !ok {
					disqualify(a)
				}
			}
		}
	default:
		for _, a := range in.Args {
			disqualify(a)
		}
	}
}
