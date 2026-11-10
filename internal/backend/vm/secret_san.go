package vm

// SecretSan labels every value derived from a std/secret classify and aborts when a label reaches a sink.
// Each object owns its label; an aggregate's label is joined at construction and at every write, never recomputed.

// shadow is a label's content part: lo/hi mark the secret-dependent bits of an integer, coarse a dependence not tracked by bit.
type shadow struct {
	lo, hi uint64
	coarse bool
}

func (s shadow) tainted() bool { return s.coarse || s.lo != 0 || s.hi != 0 }

func (s shadow) merge(o shadow) shadow {
	return shadow{lo: s.lo | o.lo, hi: s.hi | o.hi, coarse: s.coarse || o.coarse}
}

var secretCoarse = shadow{coarse: true}

// label is one object's state: elem covers its content, shape its length or variant tag.
type label struct {
	elem  shadow
	shape bool
}

func (l label) merge(o label) label {
	return label{elem: l.elem.merge(o.elem), shape: l.shape || o.shape}
}

func (l label) tainted() bool { return l.shape || l.elem.tainted() }

// whole is the label of a use that observes both the content and the shape.
func (l label) whole() shadow {
	if l.shape {
		return l.elem.merge(secretCoarse)
	}
	return l.elem
}

// hasShape reports a kind whose length, tag or Go-built parts a shape label covers; a scalar keeps bit precision instead.
func hasShape(k kind) bool {
	switch k {
	case kFree, kInt, kFloat, kBool, kChar, kUnit:
		return false
	}
	return true
}

// fullShadow labels every bit of an integer of kind nk.
func fullShadow(nk int) shadow {
	if is128(nk) {
		return shadow{lo: ^uint64(0), hi: ^uint64(0)}
	}
	if b := bitsOf(nk); b < 64 {
		return shadow{lo: uint64(1)<<b - 1}
	}
	return shadow{lo: ^uint64(0)}
}

// maskTo trims a bit shadow to the width of nk.
func maskTo(s shadow, nk int) shadow {
	if is128(nk) {
		return s
	}
	if b := bitsOf(nk); b < 64 {
		m := uint64(1)<<b - 1
		return shadow{lo: s.lo & m, hi: 0, coarse: s.coarse}
	}
	return shadow{lo: s.lo, hi: 0, coarse: s.coarse}
}
