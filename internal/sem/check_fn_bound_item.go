package sem

import "kigumi/internal/syntax"

func fnItemKind(info *FnInfo) string {
	switch {
	case info.Declared&EffUnsafe != 0:
		return "an `unsafe fn`"
	case info.Abi != "":
		return "an `extern(" + info.Abi + ")` function"
	case info.Naked:
		return "a naked function"
	}
	return ""
}

func isFnKind(k ConstraintKind) bool {
	return k == CFn || k == CFnMut || k == CFnOnce
}

func (c *checker) rejectFnItemBound(n, a syntax.NodeID, fn EntityID, i int) {
	tt := c.r.Types
	pt := tt.Node(c.r.Fn(fn).Sig).Args[i]
	for _, con := range c.r.typeParam(tt.Node(pt).Ent).Constraints {
		if !isFnKind(con.Kind) {
			continue
		}
		for _, item := range c.argFnItems(a) {
			if kind := fnItemKind(c.r.Fn(item)); kind != "" {
				c.errAt(a, cUnsafeFnBound, c.r.Entities[item].Name, kind, c.constraintText(con))
				c.markUnsatisfied(n, c.vars.resolve(c.info.Types[a]), 0)
				return
			}
		}
	}
}

func (c *checker) argFnItems(a syntax.NodeID) []EntityID {
	if u := c.info.Uses[a]; u != 0 && c.r.Entities[u].Kind == EntFn {
		return []EntityID{u}
	}
	var items []EntityID
	for _, item := range c.localKnownFns(a) {
		if c.r.Entities[item].Kind == EntFn {
			items = append(items, item)
		}
	}
	return items
}

func (c *checker) fnValueKind(arg TypeID) string {
	if c.r.Types.Kind(arg) != KFn {
		return ""
	}
	flags := c.r.Types.Node(arg).Flags
	switch {
	case flags&uint16(EffUnsafe) != 0:
		return "an `unsafe fn` value"
	case flags&fnCAbi != 0:
		return "a C function pointer"
	}
	return ""
}

func (c *checker) rejectFnValueBound(n syntax.NodeID, arg TypeID, con Constraint) bool {
	if c.alreadyUnsatisfied(n, arg, 0) {
		return true
	}
	kind := c.fnValueKind(arg)
	if kind == "" {
		return false
	}
	c.errAt(n, cUnsafeFnBound, c.typeText(arg), kind, c.constraintText(con))
	return true
}
