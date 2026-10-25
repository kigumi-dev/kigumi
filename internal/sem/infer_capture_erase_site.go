package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// eraseSite is where a value checked against a plain fn type goes; it is marked before the value is checked.
type eraseSite uint8

const (
	siteStored eraseSite = iota
	siteLent
	siteLocal
)

// markEraseSite marks n and every value leaf of it (if / match arms, block tails, `move`).
func (c *checker) markEraseSite(n syntax.NodeID, site eraseSite) {
	if c.eraseSites == nil {
		c.eraseSites = map[syntax.NodeID]eraseSite{}
	}
	c.eraseSites[n] = site
	for _, leaf := range c.tailLeaves(n, 0) {
		c.eraseSites[leaf] = site
		if c.t.Kind(leaf) == syntax.MoveExpr {
			c.eraseSites[c.t.Unparen(c.t.View(leaf).AsMove().Operand().ID())] = site
		}
	}
}

func (c *checker) noteLentArg(a syntax.NodeID, target EntityID, param TypeID, marked bool) {
	if c.nonEscapingArg(target, param, marked) {
		c.markEraseSite(a, siteLent)
	}
}

// createdHere: the closure is built in the body being checked, so MIR of that body follows its value wherever it goes.
func (c *checker) createdHere(closure EntityID) bool {
	body := c.fn
	if len(c.lambdas) > 0 {
		body = c.lambdas[len(c.lambdas)-1].ent
	}
	return c.r.closure(closure).Creator == body
}

func (c *checker) writtenInClosure(ent EntityID) bool {
	for _, ents := range c.held.closureWrites {
		if slices.Contains(ents, ent) {
			return true
		}
	}
	return false
}

// reachesClosureWrite: running a value of t may read a cell that some closure writes.
func (c *checker) reachesClosureWrite(t TypeID, seen map[EntityID]bool) bool {
	n := c.r.Types.Node(c.vars.resolve(t))
	if n.Kind != KClosure || n.Ent == 0 || c.r.Entities[n.Ent].Kind != EntClosure || seen[n.Ent] {
		return false
	}
	seen[n.Ent] = true
	for _, cp := range c.r.closure(n.Ent).Captures {
		if cp.Mode == capCell && c.writtenInClosure(cp.Local) || c.reachesClosureWrite(c.r.Entities[cp.Local].Type, seen) {
			return true
		}
	}
	return false
}
