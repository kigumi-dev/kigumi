package vm

import "math/bits"

// Native's kg.box helper allocates a header and a slot array per aggregate and a box per scalar leaf except Bool.
func boxTicks(v *obj) int {
	v = deref(v)
	if v == nil {
		return 0
	}
	switch v.k {
	case kInt, kFloat, kChar:
		return 1
	case kRecord, kVariant:
		if len(v.fields) == 0 && v.k == kVariant {
			return 0
		}
		return 2 + leafTicks(v.fields)
	case kClosure:
		if v.env == nil || len(v.env.fields) == 0 {
			return 2
		}
		return 3 + leafTicks(v.env.fields)
	case kArray:
		if len(v.fields) == 0 {
			return 1
		}
		return 1 + bits.Len(uint(len(v.fields)-1)) + 1 + leafTicks(v.fields)
	}
	return 0
}

func leafTicks(fs []*obj) int {
	n := 0
	for _, f := range fs {
		n += boxTicks(f)
	}
	return n
}
