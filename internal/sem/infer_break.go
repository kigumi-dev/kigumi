package sem

import "kigumi/internal/syntax"

func (c *checker) synthBreak(n syntax.NodeID) TypeID {
	kw := c.t.View(n).Tok().Text()
	if c.loop == 0 && c.cleanup > 0 {
		c.errAt(n, cCleanupControlFlow, "`"+kw+"`")
	} else if c.loop == 0 {
		c.errAt(n, cBreakOutsideLoop, kw)
		return TyPoison
	}
	if kw == "break" {
		c.brokeOut = true
		val := c.t.View(n).AsBreak().Value().ID()
		if val != 0 && c.cleanup == 0 {
			c.reserveUse(val)
		}
		switch {
		case c.breaks != nil && val != 0 && c.isNoneName(val):
			c.breaks.bareNone = append(c.breaks.bareNone, val)
		case c.breaks != nil:
			t, node := TyUnit, n
			if val != 0 {
				t, node = c.synth(val), val
			}
			c.breaks.nodes = append(c.breaks.nodes, node)
			c.breaks.types = append(c.breaks.types, t)
		case val != 0:
			c.synth(val)
			c.errAt(n, cBreakValueLoopKind)
		}
	}
	c.onceLeave(kw == "break")
	return TyNever
}
