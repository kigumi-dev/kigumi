package llgen

func scalarBytes(t Type) int64 {
	switch t {
	case TI1, TI8:
		return 1
	case TI16:
		return 2
	case TI32, TFloat:
		return 4
	case TI128:
		return 16
	}
	return 8
}

// A 128-bit field is two words so every target lays it out the same way, whatever alignment the target gives i128.
const (
	wideSlot  = "[2 x i64]"
	wideAlign = 8
)

func roundUp(n, a int64) int64 { return (n + a - 1) / a * a }

func fieldBytes(f aggField) (int64, int64) {
	switch {
	case f.Unit:
		return 0, 1
	case f.Agg != nil:
		return f.Agg.Size, f.Agg.Align
	case f.Ref:
		return 8, 8
	}
	b := scalarBytes(f.mem())
	if f.mem() == TI128 {
		return b, wideAlign
	}
	return b, b
}

func maxI64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
