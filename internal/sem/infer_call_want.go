package sem

import "kigumi/internal/syntax"

func (c *checker) wantBeforeLambdas(pass int, ret, want TypeID, args []syntax.NodeID) {
	if pass != 1 || want == 0 || !c.r.Types.ContainsVar(c.vars.resolve(ret)) {
		return
	}
	for _, a := range args {
		if c.lambdaArg(a) && c.vars.unifiable(ret, want) {
			c.vars.unify(ret, want)
			return
		}
	}
}
