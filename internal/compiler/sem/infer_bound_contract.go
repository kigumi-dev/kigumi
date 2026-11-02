package sem

import (
	"cmp"

	"kigumi/internal/compiler/syntax"
)

// ownerSubst maps the type parameters of fn's owner to receiver recv's arguments, and returns fn's own parameters.
func (c *checker) ownerSubst(fn EntityID, recv TypeID) (map[EntityID]TypeID, []EntityID) {
	subst := map[EntityID]TypeID{}
	own := c.r.Fn(fn).TypeParams
	owner := c.r.ownerParams(fn)
	if recv == 0 || len(owner) == 0 || len(own) < len(owner) || own[0] != owner[0] {
		return subst, own
	}
	args := c.r.Types.Node(recv).Args
	for i, p := range owner {
		if i < len(args) {
			subst[p] = args[i]
		}
	}
	return subst, own[len(owner):]
}

// callSubst is the substitution a call instantiates fn with, whether its type arguments were inferred or written.
func (c *checker) callSubst(fn EntityID, inst []TypeID, recv TypeID) map[EntityID]TypeID {
	subst, own := c.ownerSubst(fn, recv)
	for i, p := range own {
		if i < len(inst) {
			subst[p] = inst[i]
		}
	}
	return subst
}

// slotContracts holds the value at n, placed in a slot declared as type d under subst, to every Fn bound of a type
// parameter at a position of d, and of the parameters those bounds name in turn.
func (c *checker) slotContracts(n syntax.NodeID, d TypeID, subst map[EntityID]TypeID, at erasePos) {
	c.slotContractsMode(n, d, subst, at, false)
}

// slotContractsMode is slotContracts; ifaceOnly skips a type parameter's Fn bounds, for a call through a value where a
// coercion already proves those, leaving only the interface bounds this walk must prove.
func (c *checker) slotContractsMode(n syntax.NodeID, d TypeID, subst map[EntityID]TypeID, at erasePos, ifaceOnly bool) {
	w := &slotWalk{c: c, subst: subst, seen: map[EntityID]bool{}, ifaceOnly: ifaceOnly}
	w.walk(d, at)
	c.recordWalk(n, w)
}

func (c *checker) recordWalk(n syntax.NodeID, w *slotWalk) {
	targets := c.valueTargets(n, c.vars.resolve(c.info.Types[n]))
	switch {
	case w.refuse && len(targets) == 1:
		c.refuseErasure(n, targets[0])
	case w.refuse:
		c.refuseErasure(n, 0)
	default:
		c.recordErasures(n, targets, w.slots)
	}
}

// slotWalk follows a declared slot type the way the erasure match does; refuse marks an erasure no proof can reach.
type slotWalk struct {
	c         *checker
	subst     map[EntityID]TypeID
	seen      map[EntityID]bool
	slots     []eraseSlot
	refuse    bool
	ifaceOnly bool
}

func (w *slotWalk) walk(d TypeID, at erasePos) {
	tt := w.c.r.Types
	n := tt.Node(d)
	switch n.Kind {
	case KParam:
		w.param(n.Ent, at)
	case KFn:
		for _, p := range n.Args {
			w.walk(p, erasePos{pos: !at.pos})
		}
		at.results++
		w.walk(n.Elem, at)
	case KNamed:
		for i, a := range n.Args {
			next := at
			next.path = at.path.arg(i)
			next.provable = at.provable && at.results == 0 && w.c.r.holdsArg(n.Ent, i, map[EntityID]bool{}) && w.c.r.holdsElements(d)
			w.walk(a, next)
		}
	case KIface:
		for i, a := range n.Args {
			next := at
			next.path, next.provable = at.path.arg(i), false
			w.walk(a, next)
		}
	case KRef:
		at.provable = false
		w.walk(n.Elem, at)
	}
}

func (w *slotWalk) param(p EntityID, at erasePos) {
	c, tt := w.c, w.c.r.Types
	if w.seen[p] {
		return
	}
	w.seen[p] = true
	defer delete(w.seen, p)
	val := c.vars.resolve(tt.Subst(tt.Param(p), w.subst))
	for _, con := range c.r.typeParam(p).Constraints {
		if !isFnKind(con.Kind) || tt.Kind(con.Type) != KFn {
			if con.Type != 0 {
				w.walk(con.Type, erasePos{pos: at.pos, path: at.path, results: at.results})
				w.implBounds(val, c.vars.resolve(tt.Subst(con.Type, w.subst)), at)
			}
			continue
		}
		if w.ifaceOnly {
			continue
		}
		want := c.vars.resolve(tt.Subst(con.Type, w.subst))
		if shape := c.valueShape(val, want); shape != 0 && at.pos {
			if slots, ok := c.fitSlots(shape, want); ok {
				for _, s := range slots {
					w.refuse = w.refuse || !at.provable || len(s.path) > 0 && at.results > 0
					w.slots = append(w.slots, eraseSlot{at.path.then(s.path), at.results + s.results, s.slot})
				}
			}
		}
		w.walk(con.Type, at)
	}
}

// valueShape is the fn type a value of type val is measured on against want; 0 when it is not a callable or a type
// parameter one of whose bounds fits want without an erasure.
func (c *checker) valueShape(val, want TypeID) TypeID {
	switch c.r.Types.Kind(val) {
	case KFn:
		return val
	case KClosure:
		return c.r.closure(c.r.Types.Node(val).Ent).Sig
	case KParam:
		return c.boundToErase(val, want)
	}
	return 0
}

// paramShapes are the fn types a value of type parameter t is known to be: one for each Fn bound.
func (c *checker) paramShapes(t TypeID) []TypeID {
	var out []TypeID
	for _, con := range c.r.typeParam(c.r.Types.Node(t).Ent).Constraints {
		if isFnKind(con.Kind) && c.r.Types.Kind(con.Type) == KFn {
			out = append(out, con.Type)
		}
	}
	return out
}

// boundToErase is the Fn bound of type parameter t a coercion to want goes through: 0 when one bound fits without an
// erasure, else the first that fits by one.
func (c *checker) boundToErase(t, want TypeID) TypeID {
	var first TypeID
	for _, s := range c.paramShapes(t) {
		slots, ok := c.fitSlots(s, want)
		if ok && len(slots) == 0 {
			return 0
		}
		if ok && first == 0 {
			first = s
		}
	}
	return cmp.Or(first, c.r.fnShapeOf(t))
}
