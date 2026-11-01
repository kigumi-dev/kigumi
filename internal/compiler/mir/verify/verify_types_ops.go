package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *verifier) binaryMismatch(in midir.Inst, l, r, dst sem.TypeID) string {
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

func yieldsData(op midir.Opcode) bool {
	switch op {
	case midir.OpConst, midir.OpBinary, midir.OpUnary, midir.OpRecord, midir.OpVariant, midir.OpArray, midir.OpField, midir.OpFieldMove, midir.OpIndex, midir.OpTag, midir.OpPayload,
		midir.OpIsVariant, midir.OpIsType, midir.OpUnbox, midir.OpBox, midir.OpClosure, midir.OpFnItem, midir.OpBind, midir.OpBorrow, midir.OpNewCell, midir.OpCellGet, midir.OpCellTake,
		midir.OpInterp, midir.OpBoxDirect, midir.OpUnboxDirect, midir.OpLendBox:
		return true
	}
	return false
}
