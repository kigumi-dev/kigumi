package sem

import "kigumi/internal/syntax"

// consumeLog records what an unconditional use moved out of a binding: all of it, or the fields at the end of a path.
type consumeLog struct {
	whole map[EntityID]bool
	parts map[EntityID][][]EntityID
}

func (l *consumeLog) markWhole(id EntityID) {
	if l.whole == nil {
		l.whole = map[EntityID]bool{}
	}
	l.whole[id] = true
}

func (l *consumeLog) addPart(id EntityID, path []EntityID) {
	if l.parts == nil {
		l.parts = map[EntityID][][]EntityID{}
	}
	l.parts[id] = append(l.parts[id], path)
}

func (l *consumeLog) clear(id EntityID) {
	delete(l.whole, id)
	delete(l.parts, id)
}

// consumePart logs a field path of a parameter moved out of it, which scope exit then no longer drops.
func (c *checker) consumePart(a syntax.NodeID) {
	p, ok := c.consumedPlace(a)
	if !ok || len(p.fields) == 0 || c.r.Entities[p.ent].Kind != EntParam {
		return
	}
	c.consumed.addPart(p.ent, p.fields)
}
