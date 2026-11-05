package llgen

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
)

// keepsUnboxed: a boxed parameter is reused for free, so a register one pays only if no copy of it is ever boxed.
func (e *emitter) keepsUnboxed(f *midir.Func, prm midir.LocalID, regs map[midir.LocalID]scalarInfo, retNative bool) bool {
	if _, ok := regs[prm]; !ok {
		return false
	}
	fam := map[midir.LocalID]bool{prm: true}
	for grew := true; grew; {
		grew = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if copyLike(in.Op) && fam[in.Args[0]] && !fam[in.Dst] {
					if _, ok := regs[in.Dst]; !ok {
						return false
					}
					fam[in.Dst], grew = true, true
				}
			}
		}
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			for i, a := range in.Args {
				if fam[a] && !e.unboxedUse(f, in, i) {
					return false
				}
			}
		}
		for _, a := range blk.Term.Args {
			if !fam[a] {
				continue
			}
			if blk.Term.Op != midir.TermBranch && (blk.Term.Op != midir.TermReturn || !retNative) {
				return false
			}
		}
	}
	return true
}

func copyLike(op midir.Opcode) bool {
	return op == midir.OpAlias || op == midir.OpShare || op == midir.OpMove
}

func (e *emitter) unboxedUse(f *midir.Func, in midir.Inst, i int) bool {
	switch in.Op {
	case midir.OpAlias, midir.OpShare, midir.OpMove, midir.OpDrop, midir.OpRelease, midir.OpInterp:
		return true
	case midir.OpBinary:
		ty, _ := e.scalarType(f.Locals[in.Args[0]].Type)
		rty, _ := e.scalarType(f.Locals[in.Args[1]].Type)
		return (ty == rty || wideShiftOperands(in.Str, ty, rty)) && nativeBinary(in.Str, ty)
	case midir.OpUnary:
		ty, _ := e.scalarType(f.Locals[in.Args[0]].Type)
		return in.Str == "!" && ty == TI1 || in.Str == "-" && (ty == TI64 || ty == TI128 || ty == TDouble || ty == TFloat) || in.Str == "~" && (ty == TI64 || ty == TI128)
	case midir.OpCall:
		_, ok := e.calleeScalarSig(in).param(i)
		return in.DPrim != opdef.DPNone || ok || e.wideCallOp(f, in) != "" || e.rawConvert(f, in) || e.cNative(in) && e.cArgInReg(in, i)
	case midir.OpRecord, midir.OpVariant, midir.OpClosure:
		return e.isDirect(in.Dst)
	case midir.OpSetField, midir.OpIndex, midir.OpSetIndex:
		return i > 0 && e.isDirect(in.Args[0])
	case midir.OpBuiltin:
		return e.inPlaceBuiltin(in)
	}
	return false
}

// nativeBinary mirrors the operators nativeBinaryOp computes without a box.
func nativeBinary(op string, ty Type) bool {
	switch op {
	case "==", "!=":
		return ty != TVoid
	case "<", "<=", ">", ">=", "+", "-", "*", "/":
		return ty == TI64 || ty == TI128 || ty == TDouble || ty == TFloat
	case "%", "&", "|", "^", "<<", ">>":
		return ty == TI64 || ty == TI128
	}
	return false
}

// wideShiftOperands: a 128-bit integer is shifted by a usize, which is the one binary operator whose operands differ in kind.
func wideShiftOperands(op string, l, r Type) bool {
	return (op == "<<" || op == ">>") && l == TI128 && r == TI64
}
