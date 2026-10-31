package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) noteNoallocErase(n syntax.NodeID, want TypeID) {
	if Effects(c.r.Types.Node(c.vars.resolve(want)).Flags)&EffNoalloc != 0 {
		c.r.noallocErase[literalEntKey{File: c.f, Node: c.t.Unparen(n)}] = true
	}
}

// ErasesToNoalloc reports whether the fn item at n is coerced to a `noalloc fn` type or bound.
func (r *Result) ErasesToNoalloc(file FileID, n syntax.NodeID) bool {
	return r.noallocErase[literalEntKey{File: file, Node: n}]
}
