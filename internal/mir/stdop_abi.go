package mir

type ResultABI uint8

const (
	ResultBoxed ResultABI = iota
	ResultDirect
)

type DirectPrim uint8

const (
	DPNone DirectPrim = iota
	DPLen
	DPArrayLen
	DPBytesGet
	DPArrayGet
	DPCharFromInt
	DPParseFloat
	DPConvert
	DPWrapAdd
	DPWrapSub
	DPWrapMul
	DPWrapShl
	DPWrapShr
	DPRotl
	DPRotr
	DPSqrt
	DPFloor
	DPCeil
	DPRound
	DPPow
)

// A call takes the direct form only when its result type is a register scalar or an Option of one.
var directResultOps = map[string]DirectPrim{
	"LEN":              DPLen,
	"ARRAY_LEN":        DPArrayLen,
	"BYTES_GET":        DPBytesGet,
	"ARRAY_GET":        DPArrayGet,
	"CHAR_FROM_INT":    DPCharFromInt,
	"TEXT_PARSE_FLOAT": DPParseFloat,
	"WRAPPING_ADD":     DPWrapAdd,
	"WRAPPING_SUB":     DPWrapSub,
	"WRAPPING_MUL":     DPWrapMul,
	"WRAPPING_SHL":     DPWrapShl,
	"WRAPPING_SHR":     DPWrapShr,
	"ROTATE_LEFT":      DPRotl,
	"ROTATE_RIGHT":     DPRotr,
	"MATH_SQRT":        DPSqrt,
	"MATH_FLOOR":       DPFloor,
	"MATH_CEIL":        DPCeil,
	"MATH_ROUND":       DPRound,
	"MATH_POW":         DPPow,
}

// The numeric conversions are not registry ops; isConversion finds them by key.
func (id StdOpId) ResultABI() ResultABI {
	if _, ok := directResultOps[StdOpNames[id]]; ok {
		return ResultDirect
	}
	return ResultBoxed
}

func (id StdOpId) directPrim() DirectPrim { return directResultOps[StdOpNames[id]] }
