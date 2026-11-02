package sem

import "kigumi/internal/compiler/syntax"

// rejectIndexTake applies fieldTakeRejects to a path under a bracket. An element slot is never marked
// moved, and a slice shares its source's elements, so either taken by value needs a borrow.
func (c *checker) rejectIndexTake(n syntax.NodeID, take fieldTake) bool {
	t := c.vars.resolve(c.info.Types[n])
	if t == 0 || t == TyPoison || c.copyNow(t) || c.r.Types.Kind(t) == KRef || c.r.hasErrorAt(c.f, n) || !fieldTakeRejects[take][c.pathMode(n)] {
		return false
	}
	root, _ := c.indexRoot(n)
	switch c.bracketKind(root) {
	case bracketElement:
		c.errAtHelp(n, c.indexValueHelp(n, t), cIndexValueCopy, t)
	case bracketSlice:
		c.errAtHelp(n, c.sliceHelp(root, "borrow it with `&"+c.takeName(n)+"`"), cIndexValueCopy, t)
	default:
		return false
	}
	return true
}

// sliceHelp names the source a slice shares its elements with; Copy elements can also be copied out with `.slice`.
func (c *checker) sliceHelp(slice syntax.NodeID, fix string) string {
	if elem, ok := c.r.arrayElem(c.vars.resolve(c.info.Types[slice])); ok && c.copyNow(elem) {
		fix += ", or copy the elements with `.slice`"
	}
	return "`" + c.takeName(slice) + "` shares its elements with `" + c.takeName(c.sliceSource(slice)) + "`; " + fix
}
