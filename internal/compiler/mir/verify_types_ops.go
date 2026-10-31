package mir

import "kigumi/internal/compiler/sem"

func (p *Program) binaryMismatch(in Inst, l, r, dst sem.TypeID) string {
	switch in.Str {
	case "==", "!=", "<", "<=", ">", ">=":
		if dst != sem.TyBool {
			return "comparison " + in.Str + " yields " + p.R.TypeString(dst) + ", not Bool"
		}
		if !p.compatible(l, r) {
			return "comparison " + in.Str + " of " + p.R.TypeString(l) + " and " + p.R.TypeString(r)
		}
	case "..", "..=":
	case "<<", ">>":
		if !p.compatible(l, dst) {
			return "operator " + in.Str + " of " + p.R.TypeString(l) + " yields " + p.R.TypeString(dst)
		}
	default:
		if !p.compatible(l, r) || (!p.compatible(l, dst) && !p.compatible(r, dst)) {
			return "operator " + in.Str + " of " + p.R.TypeString(l) + " and " + p.R.TypeString(r) + " yields " + p.R.TypeString(dst)
		}
	}
	return ""
}

func yieldsData(op Opcode) bool {
	switch op {
	case OpConst, OpBinary, OpUnary, OpRecord, OpVariant, OpArray, OpField, OpFieldMove, OpIndex, OpTag, OpPayload,
		OpIsVariant, OpIsType, OpUnbox, OpBox, OpClosure, OpFnItem, OpBind, OpBorrow, OpNewCell, OpCellGet, OpCellTake,
		OpInterp, OpBoxDirect, OpUnboxDirect, OpLendBox:
		return true
	}
	return false
}
