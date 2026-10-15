package sem

import (
	"slices"

	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// strongWrite is an unconditional statement `p.f = v` of a function body on its by-value `mut` parameter p.
type strongWrite struct {
	param, field EntityID
	at           token.Span
}

func (c *checker) noteStrongFieldWrite(stmt, lhs syntax.NodeID) {
	if c.fn == 0 || len(c.lambdas) > 0 {
		return
	}
	if body := c.r.Fn(c.fn).Body; body == 0 || !slices.Contains(c.t.List(body).AppendIDs(nil), stmt) {
		return
	}
	p, ok := c.buildPlace(lhs)
	if !ok || len(p.Fields) != 1 {
		return
	}
	root := c.r.canonEnt(p.Root)
	e := &c.r.Entities[root]
	if e.Kind != EntParam || e.Parent != c.fn || e.Flags&EfMut == 0 || c.r.Types.Kind(c.vars.resolve(e.Type)) == KRef {
		return
	}
	if c.r.flow.strongWrites == nil {
		c.r.flow.strongWrites = map[EntityID][]strongWrite{}
	}
	c.r.flow.strongWrites[c.fn] = append(c.r.flow.strongWrites[c.fn], strongWrite{param: root, field: p.Fields[0], at: c.t.Span(stmt)})
}

// overwrittenBefore lists param's fields rewritten by a top-level statement that ends before leaf starts.
func (c *checker) overwrittenBefore(param EntityID, leaf syntax.NodeID) []EntityID {
	var out []EntityID
	for _, w := range c.r.flow.strongWrites[c.fn] {
		if w.param == param && w.at.End <= c.t.Span(leaf).Start && !slices.Contains(out, w.field) {
			out = append(out, w.field)
		}
	}
	return out
}

func (r *Result) skipSetID(fields []EntityID) int {
	if len(fields) == 0 {
		return 0
	}
	for i, s := range r.flow.skipSets {
		if slices.Equal(s, fields) {
			return i + 1
		}
	}
	r.flow.skipSets = append(r.flow.skipSets, slices.Clone(fields))
	return len(r.flow.skipSets)
}
