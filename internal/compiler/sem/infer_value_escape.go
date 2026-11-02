package sem

import "kigumi/internal/compiler/syntax"

const (
	usePassed = "only a direct call through the value can prove its argument, and this use passes it on"
	useMixed  = "this call goes through a local that may hold other values, so it cannot prove the argument"
)

func (c *checker) escapeHolder(node syntax.NodeID, r *holderRes, use string) {
	if r.escaped {
		return
	}
	r.escaped = true
	for _, d := range r.vals {
		c.escapeValueData(node, d, use)
	}
}

func (c *checker) escapeValueData(node syntax.NodeID, d *valueInstData, use string) {
	if d.escapeAt == 0 {
		d.escapeAt, d.escapeUse = node, use
	}
}

type valueIfaceCheck struct {
	cc         constraintCheck
	arg, iface TypeID
}

// flushValueIfaceChecks runs once the whole body is checked. A value that escaped is refused at that use; one proved by
// direct calls, or never used, needs nothing more. A value the escape walk never met is refused where it is made.
func (c *checker) flushValueIfaceChecks() {
	checks := c.valueIfaceChecks
	c.valueIfaceChecks = nil
	for _, v := range checks {
		c.valueIfaceErasure(v.cc, v.arg, v.iface)
	}
}

func (c *checker) valueIfaceErasure(cc constraintCheck, arg, iface TypeID) {
	w := &slotWalk{c: c, seen: map[EntityID]bool{}}
	w.implBounds(arg, iface, slotRoot)
	if !w.refuse && len(w.slots) == 0 {
		return
	}
	switch d := cc.held; {
	case d != nil && d.escapeAt != 0:
		c.refuseErasureUse(d.escapeAt, c.valueItem(cc), d.escapeUse)
	case d != nil && (d.called || d.seen):
	default:
		c.refuseErasure(cc.node, c.valueItem(cc))
	}
}
