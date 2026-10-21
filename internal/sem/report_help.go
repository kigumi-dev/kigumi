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
