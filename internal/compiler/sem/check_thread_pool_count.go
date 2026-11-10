package sem

import (
	"math/big"

	"kigumi/internal/compiler/syntax"
)

// threadPoolMax is the most workers a pool executor may start; the runtime
// refuses the same bound, so `check` must not wave a literal past it.
var threadPoolMax = big.NewInt(256)

// threadPoolCountCheck rejects a literal worker count outside 1 to 256. The
// constructor has no Err form: a bad count aborts the process at run time, so
// the bound has to be a diagnostic, as the exit-code range is (E2207).
func (c *checker) threadPoolCountCheck(fn EntityID, args []syntax.NodeID) {
	if len(args) != 1 || c.r.entityByStdKey(threadPoolKey) != fn {
		return
	}
	if !isNumericLiteral(c.t, args[0]) || c.literalValue(args[0]) == TyPoison {
		return
	}
	v := c.info.Literals[args[0]]
	if v.Kind != constInt || (v.Int.Sign() > 0 && v.Int.Cmp(threadPoolMax) <= 0) {
		return
	}
	c.errAt(args[0], cThreadPoolCount, v.Int.String())
}
