package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

type fieldMoveSite struct {
	node, field syntax.NodeID
	recv        TypeID
	loops       []syntax.NodeID
}

func (c *checker) noteFieldMoveSite(root EntityID, node, field syntax.NodeID, recv TypeID) {
	if root == 0 || len(c.methodValues.loops) == 0 {
		return
	}
	if c.methodValues.fieldMoves == nil {
		c.methodValues.fieldMoves = map[EntityID][]fieldMoveSite{}
	}
	site := fieldMoveSite{node: node, field: field, recv: recv, loops: slices.Clone(c.methodValues.loops)}
	c.methodValues.fieldMoves[root] = append(c.methodValues.fieldMoves[root], site)
}

func (c *checker) recheckLoopFieldMoves(ent, src EntityID, assign syntax.NodeID) {
	name, owner := c.r.Entities[ent].Name, c.r.Entities[src].Name
	for _, s := range c.methodValues.fieldMoves[ent] {
		if !slices.ContainsFunc(s.loops, func(l syntax.NodeID) bool {
			return slices.Contains(c.methodValues.loops, l) && !c.declaredInLoop(ent, l)
		}) {
			continue
		}
		var args []any
		code := cMoveAliasedField
		if s.recv != 0 {
			code = cMethodValuePlace
			args = []any{s.recv, "a field of `" + name + "`, which aliases the matched `" + owner + "` on the next pass of this loop", "destructure `" + owner + "` with `let ... else` to give it up, or call the method directly"}
		} else {
			args = []any{c.placeName(s.field), name, owner}
		}
		d, ok := c.r.diagAt(c.f, s.node, code, args...)
		if !ok {
			continue
		}
		c.r.emitDiag(c.f, d.WithNote(c.loc(assign), "this assignment makes `"+name+"` an alias of the matched `"+owner+"`"))
	}
	delete(c.methodValues.fieldMoves, ent)
}

func (c *checker) declaredInLoop(ent EntityID, loop syntax.NodeID) bool {
	e := &c.r.Entities[ent]
	if e.File != c.f || e.Node == 0 {
		return false
	}
	at, span := c.t.Span(e.Node), c.t.Span(loop)
	return span.Start <= at.Start && at.End <= span.End
}
