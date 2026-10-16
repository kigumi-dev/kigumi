package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

type heldResult struct {
	roots    []EntityID
	lent     map[EntityID]EntityID
	conflict EntityID
	hit      EntityID
	lost     bool
	lostNode syntax.NodeID
	lostWhat string
	opaque   bool
	seen     map[TypeID]bool
}

func newHeldResult(roots []EntityID, lent map[EntityID]EntityID) *heldResult {
	return &heldResult{roots: roots, lent: lent, seen: map[TypeID]bool{}}
}

func (h *heldResult) sub() *heldResult {
	return &heldResult{roots: h.roots, lent: h.lent, seen: h.seen}
}

func (h *heldResult) lose(n syntax.NodeID, what string) {
	if !h.lost {
		h.lost, h.lostNode, h.lostWhat = true, n, what
	}
}

func (h *heldResult) merge(o *heldResult) {
	if h.conflict == 0 {
		h.conflict, h.hit = o.conflict, o.hit
	}
	if o.lost {
		h.lose(o.lostNode, o.lostWhat)
	}
}

// A type parameter can call back only through a Fn bound or an interface bound.
func (r *Result) paramCanCall(ent EntityID) bool {
	for _, con := range r.typeParam(ent).Constraints {
		switch con.Kind {
		case CFn, CFnMut, CFnOnce, CIface:
			return true
		}
	}
	return false
}

// A plain `fn` part is skipped: erasing a closure to `fn` already rejects cell and move captures.
func (c *checker) scanHeld(t TypeID, h *heldResult) {
	t = c.vars.resolve(t)
	if t == 0 || h.seen[t] || h.conflict != 0 {
		return
	}
	h.seen[t] = true
	defer delete(h.seen, t)
	tt := c.r.Types
	n := tt.Node(t)
	switch n.Kind {
	case KClosure:
		if n.Ent != 0 {
			c.closureHolds(n.Ent, h)
		}
	case KIface, KVar:
		h.opaque = true
	case KParam:
		if c.r.paramCanCall(n.Ent) {
			h.opaque = true
		}
	case KRef:
		c.scanHeld(n.Elem, h)
	case KNamed:
		for _, a := range n.Args {
			c.scanHeld(a, h)
		}
		c.scanHeldFields(n, h)
	}
}

func (c *checker) scanHeldFields(n typeNode, h *heldResult) {
	info := c.r.typeDecl(n.Ent)
	subst := instSubst(info.Params, n.Args)
	switch info.Form {
	case FormRecord, FormResource:
		for _, ft := range c.r.fieldTypes(info, subst) {
			c.scanHeld(ft, h)
		}
	case FormAdt:
		for _, v := range info.Variants {
			for _, pt := range c.r.payloadTypes(v, subst) {
				c.scanHeld(pt, h)
			}
		}
	case FormDefined:
		c.scanHeld(c.r.Types.Subst(info.Underlying, subst), h)
	}
}

func (c *checker) closureHolds(closure EntityID, h *heldResult) {
	for _, cap := range c.r.closure(closure).Captures {
		if slices.Contains(h.roots, cap.Local) {
			if cap.Mode != capCopy && !c.closureAvoidsField(closure, cap.Local, h.lent[cap.Local]) {
				h.conflict, h.hit = closure, cap.Local
				return
			}
			continue
		}
		sub := h.sub()
		c.scanHeld(c.r.Entities[cap.Local].Type, sub)
		if sub.conflict != 0 {
			h.merge(sub)
			return
		}
		if sub.opaque {
			c.traceLocal(cap.Local, h)
		}
	}
}

func (c *checker) traceLocal(ent EntityID, h *heldResult) {
	if init, ok := c.letInit[ent]; ok {
		c.heldOf(init, h, map[syntax.NodeID]bool{})
		return
	}
	if e := &c.r.Entities[ent]; e.Kind == EntLocal {
		h.lose(e.Node, "`"+e.Name+"`")
	}
}
