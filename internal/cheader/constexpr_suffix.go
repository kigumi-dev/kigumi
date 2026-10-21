package cheader

import "math/big"

var suffixMacros = map[string]bool{
	"INT8_C": true, "INT16_C": true, "INT32_C": true, "INT64_C": true, "INTMAX_C": true,
	"UINT8_C": true, "UINT16_C": true, "UINT32_C": true, "UINT64_C": true, "UINTMAX_C": true,
	"__INT64_C": true, "__UINT64_C": true,
}

var (
	maxInt64  = new(big.Int).SetInt64(1<<63 - 1)
	minInt64  = new(big.Int).SetInt64(-1 << 63)
	maxUint32 = new(big.Int).SetInt64(1<<32 - 1)
	maxUint64 = new(big.Int).SetUint64(1<<64 - 1)
)

func (p *cparser) suffixed(name string, arg cval) cval {
	v := arg.v
	switch name {
	case "INT8_C", "INT16_C", "INT32_C", "UINT8_C", "UINT16_C":
		return arg.promoted()
	case "UINT32_C":
		switch {
		case v.Sign() < 0, v.Cmp(maxUint64) > 0:
			return p.fail()
		case v.Cmp(maxUint32) <= 0:
			return cval{v: v, bits: 32}
		}
		return cval{v: v, bits: 64}
	case "INT64_C", "INTMAX_C", "__INT64_C":
		switch {
		case v.Cmp(maxInt64) <= 0 && v.Cmp(minInt64) >= 0:
			return cval{v: v, bits: 64, signed: true}
		case v.Sign() > 0 && v.Cmp(maxUint64) <= 0:
			return cval{v: v, bits: 64}
		}
	default:
		if v.Sign() >= 0 && v.Cmp(maxUint64) <= 0 {
			return cval{v: v, bits: 64}
		}
	}
	return p.fail()
}
