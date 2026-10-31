package vm

type bop uint8

const (
	bopOther bop = iota
	bopAdd
	bopSub
	bopMul
	bopDiv
	bopRem
	bopAnd
	bopOr
	bopXor
	bopShl
	bopShr
	bopLt
	bopLe
	bopGt
	bopGe
	bopEq
	bopNe
)

var bopNames = map[string]bop{
	"+": bopAdd, "-": bopSub, "*": bopMul, "/": bopDiv, "%": bopRem,
	"&": bopAnd, "|": bopOr, "^": bopXor, "<<": bopShl, ">>": bopShr,
	"<": bopLt, "<=": bopLe, ">": bopGt, ">=": bopGe, "==": bopEq, "!=": bopNe,
}

func bopOf(op string) bop { return bopNames[op] }
