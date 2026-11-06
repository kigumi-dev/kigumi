package ops

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
	DPArrayFirst
	DPArrayLast
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
	DPTryRef
	DPTryOpt
	DPMapSeed
	DPSecretBarrier
	DPSecretMaskNonZero
	DPSecretMaskLt
	DPSecretSelect
)

var directPrimByName = map[string]DirectPrim{
	"Len": DPLen, "ArrayLen": DPArrayLen, "BytesGet": DPBytesGet, "ArrayGet": DPArrayGet,
	"ArrayFirst": DPArrayFirst, "ArrayLast": DPArrayLast, "CharFromInt": DPCharFromInt, "ParseFloat": DPParseFloat,
	"WrapAdd": DPWrapAdd, "WrapSub": DPWrapSub, "WrapMul": DPWrapMul, "WrapShl": DPWrapShl, "WrapShr": DPWrapShr,
	"Rotl": DPRotl, "Rotr": DPRotr, "Sqrt": DPSqrt, "Floor": DPFloor, "Ceil": DPCeil, "Round": DPRound, "Pow": DPPow,
	"TryRef": DPTryRef, "TryOpt": DPTryOpt, "MapSeed": DPMapSeed,
	"SecretBarrier": DPSecretBarrier, "SecretMaskNonZero": DPSecretMaskNonZero,
	"SecretMaskLt": DPSecretMaskLt, "SecretSelect": DPSecretSelect,
}

// A call takes the direct form only when its result type is a register scalar, an Option of one, or a direct Result[T, AllocError].
var directResultOps = func() map[string]DirectPrim {
	m := map[string]DirectPrim{}
	for _, op := range Registry.Ops {
		if op.Direct != "" {
			m[op.Name] = directPrimByName[op.Direct]
		}
	}
	return m
}()

// The numeric conversions are not registry ops; isConversion finds them by key.
func (id StdOpId) ResultABI() ResultABI {
	if _, ok := directResultOps[StdOpNames[id]]; ok {
		return ResultDirect
	}
	return ResultBoxed
}

func (id StdOpId) DirectPrimitive() DirectPrim { return directResultOps[StdOpNames[id]] }
