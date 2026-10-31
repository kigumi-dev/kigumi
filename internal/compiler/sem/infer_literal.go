package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) synthLiteral(n syntax.NodeID, want TypeID) TypeID {
	untyped := c.literalValue(n)
	if untyped == TyPoison || want == 0 {
		return untyped
	}
	return c.coerce(n, untyped, want)
}
