package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) stripRef(t sem.TypeID) sem.TypeID {
	for c.p.R.Types.Kind(t) == sem.KRef {
		t = c.p.R.Types.Node(t).Elem
	}
	return t
}

func (c *borrowCheck) multiLifetimeArg(in midir.Inst) sem.TypeID {
	for _, l := range append([]midir.LocalID{in.Dst}, in.Args...) {
		if t := c.stripRef(c.f.Locals[l].Type); c.multiLifetime(t) {
			return t
		}
	}
	return 0
}

// Reported once per build, not per field.
func (c *borrowCheck) checkMultiLifetimeBuild(in midir.Inst) bool {
	t := c.f.Locals[in.Dst].Type
	if !c.multiLifetime(t) {
		return false
	}
	for _, a := range in.Args {
		if b, ok := c.holders[a]; ok && !b.hasOwner {
			c.reportCode(in.Node, a, "borrow-multi-lifetime", c.p.R.TypeString(t))
			return true
		}
	}
	return false
}
