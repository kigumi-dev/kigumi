package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

type callSlot struct {
	value syntax.NodeID
	param EntityID
	recv  RecvKind
}

func (c *checker) argSlots(args []syntax.NodeID, declParams []EntityID) []callSlot {
	out := make([]callSlot, len(args))
	for j, a := range args {
		out[j] = callSlot{value: c.t.Unparen(c.t.View(a).ArgValue().ID()), param: paramAt(c.r, declParams, j)}
	}
	return out
}

func (c *checker) noteArgStores(args []syntax.NodeID, i int, declParams []EntityID) {
	slots := c.argSlots(args, declParams)
	c.noteStoresInto(slots[i], slices.Concat(slots[:i], slots[i+1:]))
}

func (c *checker) noteRecvStores(n, base syntax.NodeID, m EntityID, args []syntax.NodeID) {
	info := c.r.Fn(m)
	if base == 0 || info.SelfParam == 0 {
		return
	}
	if call, ok := c.info.Calls[n]; ok && call.Plan != nil {
		args = call.Plan.placed()
	}
	recv := callSlot{value: c.t.Unparen(base), param: info.SelfParam, recv: info.Recv}
	slots := c.argSlots(args, info.Params)
	c.noteStoresInto(recv, slots)
	for _, s := range slots {
		c.noteStoresInto(s, []callSlot{recv})
	}
}

func (c *checker) noteStoresInto(to callSlot, from []callSlot) {
	store := c.storeInto(to)
	if store == nil {
		return
	}
	for _, s := range from {
		if !c.lendsShared(s) && c.r.mayStore(to.param, s.param) {
			store(s.value)
		}
	}
}

func (c *checker) storeInto(to callSlot) func(syntax.NodeID) {
	if to.recv == RecvSelf {
		return nil
	}
	t := c.vars.resolve(c.info.Types[to.value])
	place := to.value
	switch {
	case c.t.Kind(to.value) == syntax.BorrowExpr && c.t.View(to.value).AsBorrow().Flags()&syntax.FlagMut != 0:
		place = c.t.View(to.value).AsBorrow().Operand().ID()
	case to.recv == RecvMut || c.r.Types.RefMut(t):
	case c.holdsLentHolder(t):
		starts, target := c.localSources(to.value), c.pointee(t)
		return func(v syntax.NodeID) { c.recordStoreAt(starts, reachHeld, target, v) }
	default:
		return nil
	}
	return func(v syntax.NodeID) { c.noteStoreThrough(place, v) }
}

// E2292 keeps a callee from taking a `&mut` out of a shared reference or a borrowed `self`.
func (c *checker) lendsShared(s callSlot) bool {
	t := c.vars.resolve(c.info.Types[s.value])
	return s.recv == RecvSelf || c.r.Types.Kind(t) == KRef && !c.r.Types.RefMut(t)
}

func (c *checker) localSources(expr syntax.NodeID) []lentRef {
	var out []lentRef
	for _, from := range c.leafSources(expr, 0, true, 0) {
		if from.root != 0 && (c.r.Entities[from.root].Kind == EntLocal || c.r.Entities[from.root].Kind == EntParam) {
			out = append(out, from)
		}
	}
	return out
}

// An unknown parameter or an interface's `self` may store anything.
func (r *Result) mayStore(to, from EntityID) bool {
	if to == 0 || from == 0 || r.opaqueSelf(to) || r.opaqueSelf(from) {
		return true
	}
	target := r.Entities[to].Type
	if n := r.Types.Node(target); n.Kind == KRef {
		target = n.Elem
	}
	src := r.typeParamsIn(r.Entities[from].Type, nil, 0)
	if lt := r.local(from).Lifetime; lt != 0 {
		src = append(src, lt)
	}
	return slices.ContainsFunc(r.typeParamsIn(target, nil, 0), func(p EntityID) bool { return slices.Contains(src, p) })
}

func (r *Result) opaqueSelf(p EntityID) bool {
	e := &r.Entities[p]
	return e.Flags&EfSelf != 0 && r.Types.Kind(e.Type) == KParam
}

func paramAt(r *Result, declParams []EntityID, i int) EntityID {
	if n := len(declParams); i >= n && n > 0 && r.Entities[declParams[n-1]].Flags&EfVariadic != 0 {
		return declParams[n-1]
	}
	return entAt(declParams, i)
}

func (r *Result) typeParamsIn(t TypeID, out []EntityID, depth int) []EntityID {
	n := r.Types.Node(t)
	if n.Kind == KParam {
		return append(out, n.Ent)
	}
	if depth >= 8 {
		return out
	}
	if n.Elem != 0 {
		out = r.typeParamsIn(n.Elem, out, depth+1)
	}
	for _, a := range n.Args {
		out = r.typeParamsIn(a, out, depth+1)
	}
	return out
}
