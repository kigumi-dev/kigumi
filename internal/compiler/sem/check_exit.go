package sem

import (
	"math/big"

	"kigumi/internal/compiler/syntax"
)

var exitCodeMax = big.NewInt(255)

// resolveExitItems resolves ExitError and Context for the backends, which only read lang items.
func (r *Result) resolveExitItems() {
	r.langItem(0, 0, "ExitError")
	r.langItem(0, 0, "Context")
}

// exitCodeCheck rejects a literal argument of ExitError.new outside 1 to 255.
func (c *checker) exitCodeCheck(fn EntityID, args []syntax.NodeID) {
	if len(args) != 1 || c.r.Entities[fn].Name != "new" {
		return
	}
	owner := c.r.Fn(fn).Owner
	if owner == 0 || owner != c.r.langItem(0, 0, "ExitError") {
		return
	}
	if !isNumericLiteral(c.t, args[0]) || c.literalValue(args[0]) == TyPoison {
		return
	}
	v := c.info.Literals[args[0]]
	if v.Kind != constInt || (v.Int.Sign() > 0 && v.Int.Cmp(exitCodeMax) <= 0) {
		return
	}
	c.errAt(args[0], cExitCodeRange, v.Int.String())
}
