package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) synthArgs(args []syntax.NodeID) {
	for _, a := range args {
		if v := c.piped[a]; v != 0 && c.t.Kind(a) == syntax.NamedArg {
			c.synth(v)
			continue
		}
		if k := c.t.Kind(a); k == syntax.Spread || k == syntax.NamedArg {
			c.synth(c.t.View(a).ArgValue().ID())
			continue
		}
		c.synth(a)
	}
}
