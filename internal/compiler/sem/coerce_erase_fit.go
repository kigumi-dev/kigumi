package sem

import "kigumi/internal/compiler/syntax"

// eraseSlot is a reference parameter a coercion erases, of the callables reached from the coerced value along path and
// then through results calls.
type eraseSlot struct {
	path          partPath
	results, slot int
}

// erasePos is where a walk stands: polarity, path, whether a proof can reach it. Only the value may differ in fn
// qualifiers (the coercion checks them); in storage another handle may alias, nothing may differ unproven.
type erasePos struct {
	pos, root, provable, aliased bool
	path                         partPath
	results                      int
}

// eraseFit matches got against want like unification, except that a reference parameter may differ in naming a
// lifetime: safe where the callable receives a plain reference, an erasure slot where a proof can reach the callable.
type eraseFit struct {
	v     *varStore
	slots []eraseSlot
	// lenient ignores escaping marks below the value, as unifyIgnoringEscaping does for a fn coercion.
	lenient bool
}

func (f *eraseFit) walk(g, w TypeID, at erasePos) bool {
	v, tt := f.v, f.v.r.Types
	g, w = v.resolve(g), v.resolve(w)
	ng, nw := tt.Node(g), tt.Node(w)
	switch {
	case g == w:
		return true
	case ng.Kind == KFn && nw.Kind == KFn:
		mask := ^uint16(0)
		if f.lenient {
			mask = ^fnEscaping
		}
		if !at.root && ((ng.Flags^nw.Flags)&mask != 0 || !f.lenient && ng.Var != nw.Var) {
			return false
		}
		at.root = false
		if !f.params(ng, nw, at) {
			return false
		}
		at.results++
		return f.walk(ng.Elem, nw.Elem, at)
	case (ng.Kind == KNamed || ng.Kind == KIface) && ng.Kind == nw.Kind && ng.Ent == nw.Ent && ng.Flags == nw.Flags && len(ng.Args) == len(nw.Args):
		at.root = false
		at.provable = at.provable && at.results == 0
		at.aliased = at.aliased || ng.Kind == KNamed && !v.r.holdsElements(g)
		for i := range ng.Args {
			next := at
			next.path = at.path.arg(i)
			held := ng.Kind == KNamed && v.r.holdsArg(ng.Ent, i, map[EntityID]bool{})
			if held && !f.walk(ng.Args[i], nw.Args[i], next) || !held && !v.unifyInner(ng.Args[i], nw.Args[i], f.lenient) {
				return false
			}
		}
		return true
	case ng.Kind == KRef && nw.Kind == KRef && ng.Flags == nw.Flags && ng.Flags&flagMut == 0:
		if ng.Lifetime != 0 && nw.Lifetime != 0 && !v.unifyInner(ng.Lifetime, nw.Lifetime, f.lenient) {
			return false
		}
		at.root, at.provable = false, false
		return f.walk(ng.Elem, nw.Elem, at)
	}
	return v.unifyInner(g, w, f.lenient)
}

// params matches the parameter lists of two fn types; a parameter flips polarity and leaves every proof behind.
func (f *eraseFit) params(ng, nw typeNode, at erasePos) bool {
	v, tt := f.v, f.v.r.Types
	if len(ng.Args) != len(nw.Args) || ng.Flags&fnVariadic != nw.Flags&fnVariadic {
		return false
	}
	for i := range ng.Args {
		p, q := v.resolve(ng.Args[i]), v.resolve(nw.Args[i])
		if tt.Kind(p) != KRef || tt.Kind(q) != KRef {
			if !f.walk(p, q, erasePos{pos: !at.pos, aliased: at.aliased}) {
				return false
			}
			continue
		}
		gotNamed, wantNamed := tt.RefLifetime(p) != 0, tt.RefLifetime(q) != 0
		switch {
		case gotNamed && !wantNamed && at.pos && at.provable:
			f.slots = append(f.slots, eraseSlot{at.path, at.results, i})
		case gotNamed != wantNamed && (at.aliased || gotNamed == at.pos):
			return false
		}
		if !v.unifyInner(p, q, f.lenient) {
			return false
		}
	}
	return true
}

// fitsErased binds what the match binds and keeps it only when got fits want.
func (c *checker) fitsErased(got, want TypeID, lenient bool) bool {
	mark := c.vars.mark()
	f := &eraseFit{v: c.vars, lenient: lenient}
	if f.walk(got, want, erasePos{pos: true, root: true, provable: true}) {
		return true
	}
	c.vars.rollback(mark)
	return false
}

// erasedSlots are the slots a coercion of got to want erases, measured without binding; a pair the match rejects
// falls back to the value's own parameters.
func (c *checker) erasedSlots(got, want TypeID) []eraseSlot {
	if slots, ok := c.fitSlots(got, want); ok {
		return slots
	}
	var out []eraseSlot
	for _, i := range c.r.Types.erasedRefParams(got, want) {
		out = append(out, eraseSlot{slot: i})
	}
	return out
}

func (c *checker) fitSlots(got, want TypeID) ([]eraseSlot, bool) {
	mark := c.vars.mark()
	f := &eraseFit{v: c.vars, lenient: true}
	ok := f.walk(got, want, erasePos{pos: true, root: true, provable: true})
	c.vars.rollback(mark)
	return f.slots, ok
}

// coerceErasedValue lets a value whose type differs from want only in the reference lifetimes of fn types it holds meet
// want by the same coercion, the proof going to what the value holds.
func (c *checker) coerceErasedValue(n syntax.NodeID, got, want TypeID) bool {
	tt := c.r.Types
	for _, k := range []TypeKind{tt.Kind(got), tt.Kind(want)} {
		if k == KVar || k == KFn || k == KClosure || k == KPoison {
			return false
		}
	}
	if !c.fitsErased(got, want, false) {
		return false
	}
	c.refScopeContract(n, got, want)
	return true
}
