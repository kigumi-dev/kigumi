package sem

import "kigumi/internal/syntax"

func (c *checker) errAtHelp(n syntax.NodeID, help string, code Code, args ...any) bool {
	d, ok := c.r.diagAt(c.f, n, code, args...)
	if !ok {
		return false
	}
	if help != "" {
		d.Help = []string{help}
	}
	c.r.emitDiag(c.f, d)
	return true
}

// usizeHelp names the conversion between `usize` and `Int` when those two are what disagree.
func usizeHelp(want, got TypeID) string {
	switch {
	case want == TyUsize && got == TyI64:
		return "convert with `.toUsize()`, which returns `usize?`"
	case want == TyI64 && got == TyUsize:
		return "convert with `.toInt()`, which returns `Int?`"
	}
	return ""
}

func mixedUsizeHelp(a, b TypeID) string {
	if (a == TyUsize && b == TyI64) || (a == TyI64 && b == TyUsize) {
		return "convert the `Int` with `.toUsize()` or the `usize` with `.toInt()`; both return an `Option`"
	}
	return ""
}

// returnedClosureHelp replaces the `F: Fn[...]` advice at a return type, where a generic cannot name the closure.
func (c *checker) returnedClosureHelp(want TypeID, moveOnly bool) string {
	if c.vars.resolve(want) != c.vars.resolve(c.retType) {
		return ""
	}
	if moveOnly {
		return "a returned closure cannot hold a move-only value; return a record that owns it and give the record a method"
	}
	return "a returned closure cannot own state it changes; return a record that holds the state and update it through a `mut self` method"
}

// noCloneHelp is help for a type that cannot be cloned, since a clone suggestion would be impossible to follow.
func (c *checker) noCloneHelp(t TypeID, help string) string {
	if c.r.mayHoldResource(c.vars.resolve(t), map[TypeID]bool{}) {
		return help
	}
	return ""
}

// plainFnHelp is help for a closure that cannot be a plain `fn`: at a return type a record carries the state, elsewhere the receiving parameter takes a bounded type.
func (c *checker) plainFnHelp(want TypeID, moveOnly bool) string {
	if h := c.returnedClosureHelp(want, moveOnly); h != "" {
		return h
	}
	if moveOnly {
		return "a plain `fn` cannot hold a move-only value; take the callback as `F: Fn[...]`, or `F: FnOnce[...]` and pass `move` before the closure if it consumes the value"
	}
	return "a plain `fn` cannot assign a captured variable; take the callback as `F: FnMut[...]` and declare the parameter `mut callback: F`"
}

// cellFnHelp is plainFnHelp for a closure capturing ent through a cell, worded for one that only reads it (still `FnMut`).
func (c *checker) cellFnHelp(want TypeID, closure, ent EntityID) string {
	h := c.plainFnHelp(want, false)
	if lentReceiver(c.r.Entity(ent)) || c.closureWrites(closure, ent) || c.returnedClosureHelp(want, false) != "" {
		return h
	}
	return "a plain `fn` cannot share a captured variable's cell; take the callback as `F: FnMut[...]` and declare the parameter `mut callback: F`"
}

// fnMutCallableHelp is plainFnHelp for a closure that runs a captured `FnMut` callable named name.
func (c *checker) fnMutCallableHelp(want TypeID, name string) string {
	if h := c.returnedClosureHelp(want, false); h != "" {
		return h
	}
	return "a plain `fn` cannot run the captured `FnMut` callable `" + name + "`; take the callback as `F: FnMut[...]` and declare the parameter `mut callback: F`"
}
