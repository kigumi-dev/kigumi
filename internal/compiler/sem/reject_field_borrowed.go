package sem

import "kigumi/internal/compiler/syntax"

// fieldTake is how a field leaves: by an implicit read (into a binding, aggregate, the caller, a `for`
// loop, a payload, a parameter or receiver, a pattern, a join used as a base) or an explicit `move`.
type fieldTake uint8

const (
	takeConsumed fieldTake = iota
	takeReturned
	takeIterated
	takePayload
	takeHanded
	takeMoved
	takeBound
	takeJoined
)

// fieldTakeRejects needs `move` or a borrow for an implicit read; a borrowed `for` head is reported by rejectForOfBorrowed.
var fieldTakeRejects = [...][5]bool{
	takeConsumed: {pathOwned: true, pathLent: true, pathRef: true},
	takeReturned: {pathLent: true, pathRef: true},
	takeIterated: {pathOwned: true},
	takePayload:  {pathLent: true, pathRef: true},
	takeHanded:   {pathLent: true, pathRef: true},
	takeMoved:    {},
	takeBound:    {},
	takeJoined:   {pathOwned: true, pathLent: true, pathRef: true},
}

// dropOwnerChecks applies E1021 here; a named owner elsewhere is MIR's place check, a temporary has no place there.
var dropOwnerChecks = [...][5]bool{
	takeConsumed: {pathOwned: true, pathTemp: true},
	takeReturned: {pathTemp: true},
	takeIterated: {pathOwned: true, pathTemp: true},
	takePayload:  {pathTemp: true},
	takeHanded:   {pathTemp: true},
	takeMoved:    {pathTemp: true},
	takeBound:    {pathTemp: true},
	takeJoined:   {pathOwned: true, pathTemp: true},
}

// rejectArgTake: a declared by-value parameter or `move self` receiver consumes its argument's value leaves.
func (c *checker) rejectArgTake(a syntax.NodeID, param TypeID) bool {
	return c.ownsType(param) && c.rejectOperandTake(a, 0, takeHanded)
}

// rejectPayloadTake: `||` and `?` move the payload out of their operand's value leaves.
func (c *checker) rejectPayloadTake(operand syntax.NodeID, whole TypeID) bool {
	return c.rejectOperandTake(operand, whole, takePayload)
}

// rejectOperandTake checks each value leaf a consuming operand takes. An owned field reached through
// a join is copied, not moved, into the join, so it needs `move` there as in a binding.
func (c *checker) rejectOperandTake(top syntax.NodeID, whole TypeID, take fieldTake) bool {
	rejected := false
	for _, n := range c.valueLeaves(top) {
		t := c.vars.resolve(c.info.Types[n])
		switch {
		case n != c.passThrough(top) && c.fieldMember(n) && c.pathMode(n) == pathOwned:
			rejected = c.rejectFieldLeaf(n, takeConsumed) || rejected
		case c.t.Kind(n) == syntax.Ident && take == takePayload:
			rejected = c.rejectTake(n, whole, take) || rejected
		default:
			rejected = c.rejectLeafTake(n, t, take) || rejected
		}
	}
	return rejected
}

// rejectJoinBase: a join used as a member or borrow base takes its arm values into a temporary.
func (c *checker) rejectJoinBase(base syntax.NodeID) {
	leaves := c.valueLeaves(base)
	if len(leaves) == 1 && leaves[0] == c.passThrough(base) {
		return
	}
	for _, n := range leaves {
		c.rejectFieldLeaf(n, takeJoined)
	}
}

// rejectLeafTake checks a value leaf that takes an owned value out of a place: a field read, or a
// `?.` chain whose owned payload has a custom `drop`.
func (c *checker) rejectLeafTake(n syntax.NodeID, t TypeID, take fieldTake) bool {
	switch {
	case t == 0 || !c.ownsType(t):
		return false
	case c.chainDropOwner(n) != 0:
		c.rejectDropOwnerTake(n, c.chainDropOwner(n))
		return true
	case c.indexRooted(n):
		return c.rejectIndexTake(n, take)
	case c.fieldMember(n):
		return c.rejectTake(n, t, take)
	}
	return false
}

// chainDropOwner is the custom-`drop` type a `?.` chain on an owned payload moves its result out of.
func (c *checker) chainDropOwner(n syntax.NodeID) TypeID {
	if ch := c.info.OptChains[n]; ch != nil {
		if leaf := c.t.Unparen(ch.Body); !ch.ByRef && c.fieldMember(leaf) {
			return c.pathDropOwner(leaf)
		}
		return 0
	}
	if c.t.Kind(n) != syntax.OptMemberExpr || c.info.Uses[n] == 0 || c.r.Entities[c.info.Uses[n]].Kind != EntField {
		return 0
	}
	base := c.t.View(n).AsOptMember().Base().ID()
	if elem, ok := c.r.Types.IsOption(c.vars.resolve(c.info.Types[base])); ok && !c.isPlace(base) && c.customDrop(elem) {
		return elem
	}
	return 0
}

// fieldMember is a field or `.value` read, not a method value.
func (c *checker) fieldMember(n syntax.NodeID) bool {
	if c.t.Kind(n) != syntax.MemberExpr || c.info.Uses[n] == 0 {
		return false
	}
	k := c.r.Entities[c.info.Uses[n]].Kind
	return k == EntField || k == EntType
}

// rejectTake reports a place that cannot give up what is taken from it: one reached through a borrow,
// or one under a type with a custom `drop`.
func (c *checker) rejectTake(n syntax.NodeID, t TypeID, take fieldTake) bool {
	mode := c.pathMode(n)
	owner := c.pathDropOwner(n)
	switch {
	case mode.borrowed() && fieldTakeRejects[take][mode] && !(owner != 0 && c.lentByCapture(n)):
		c.rejectBorrowedTake(n, t, take)
	case owner != 0 && (dropOwnerChecks[take][mode] || c.lentByCapture(n)):
		c.rejectDropOwnerTake(n, owner)
	default:
		return false
	}
	return true
}

func (c *checker) rejectDropOwnerTake(n syntax.NodeID, owner TypeID) {
	if !c.r.hasErrorAt(c.f, n) {
		name := c.takeName(n)
		c.errAt(n, cMoveDropType, name, owner, name)
	}
}

// lentByCapture: the path is lent because an immediate closure borrows its root binding.
func (c *checker) lentByCapture(n syntax.NodeID) bool {
	root := c.fieldRoot(n)
	return root != 0 && c.pathMode(n) == pathLent && c.r.Entities[root].Flags&EfSelf == 0 && c.lentCapture(root)
}

func (c *checker) rejectBorrowedTake(n syntax.NodeID, t TypeID, take fieldTake) {
	if c.r.hasErrorAt(c.f, n) {
		return
	}
	name := c.takeName(n)
	c.errAtHelp(n, c.borrowedTakeHelp(n, name, t, take), cFieldValueMove, name, t, name, name)
}

// borrowedTakeHelp names a fix that compiles: `move` and a reassignment for a `mut self` field, a
// `move` closure for a lent capture, matching for a payload, `&` otherwise.
func (c *checker) borrowedTakeHelp(n syntax.NodeID, name string, t TypeID, take fieldTake) string {
	root := c.fieldRoot(n)
	switch {
	case c.pathMode(n) == pathLent && root != 0 && lentReceiver(&c.r.Entities[root]) && take != takeReturned:
		return "write `move " + name + "` to take it out, and assign `" + name + "` again before the method returns"
	case c.lentByCapture(n):
		return "`" + c.r.Entities[root].Name + "` is lent to this closure; write `move` before the closure to give it `" + c.r.Entities[root].Name + "`, for a callee that takes an `FnOnce`"
	}
	help := "`" + name + "` is reached through a borrow, so it cannot be moved out; borrow it with `&" + name + "`"
	switch take {
	case takePayload:
		help = "`" + name + "` is reached through a borrow, so its payload cannot be taken out; match on it to borrow the payload instead"
	case takeHanded:
		help = "`" + name + "` is reached through a borrow, so it cannot be handed over by value; pass a value the caller owns"
	}
	if !c.r.mayHoldResource(c.vars.resolve(t), map[TypeID]bool{}) {
		help += ", or clone it"
	}
	return help
}
