package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) resultLeaves(body syntax.NodeID) []syntax.NodeID {
	return append(c.tailLeaves(body), c.exits.rets...)
}

func (c *checker) stripParen(n syntax.NodeID) syntax.NodeID {
	return c.t.Unparen(n)
}

// exitLeaves are the values `return` and `fail` hand to the caller of the function or closure being checked.
type exitLeaves struct {
	rets, fails []syntax.NodeID
}

func (c *checker) noteExitValue(val syntax.NodeID, fail bool) {
	exits := &c.exits
	if len(c.lambdas) > 0 {
		exits = &c.lambdas[len(c.lambdas)-1].exits
	}
	if fail {
		exits.fails = append(exits.fails, val)
	} else {
		exits.rets = append(exits.rets, val)
	}
}
