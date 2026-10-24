package sem

import "slices"

// consumeLog records what an unconditional use moved out of a binding: all of it, or the fields at the end of a path.
// A move stays pending until its outermost statement ends and counts only if every exit checked after it moved it too.
type consumeLog struct {
	whole   map[EntityID]bool
	parts   map[EntityID][][]EntityID
	exits   [][]movedPlace
	born    map[EntityID]int
	taking  *exitTake
	pending []pendingMove
	depth   int
}

// movedPlace is a binding, or a field path under it when path is non-empty.
type movedPlace struct {
	ent  EntityID
	path []EntityID
}

type pendingMove struct {
	movedPlace
	at int
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
	l.pending = slices.DeleteFunc(l.pending, func(p pendingMove) bool { return p.ent == id })
}

// revive forgets the moved parts an assignment to path refills or writes into, so scope exit drops them again.
func (l *consumeLog) revive(id EntityID, path []EntityID) {
	if kept := slices.DeleteFunc(l.parts[id], func(m []EntityID) bool { return fieldPathsOverlap(m, path) }); len(kept) > 0 {
		l.parts[id] = kept
	} else {
		delete(l.parts, id)
	}
	l.pending = slices.DeleteFunc(l.pending, func(p pendingMove) bool {
		return p.ent == id && len(p.path) > 0 && fieldPathsOverlap(p.path, path)
	})
}

// moved is what is out of id at this point, pending moves included: on a path that reaches here they all ran.
func (l *consumeLog) moved(id EntityID) (whole bool, parts [][]EntityID) {
	whole, parts = l.whole[id], l.parts[id]
	for _, p := range l.pending {
		switch {
		case p.ent != id:
		case len(p.path) == 0:
			whole = true
		default:
			parts = append(slices.Clip(parts), p.path)
		}
	}
	return whole, parts
}

func (l *consumeLog) commit() {
	for _, p := range l.pending {
		switch {
		case l.exitedSince(p.movedPlace, p.at):
		case len(p.path) == 0:
			l.markWhole(p.ent)
		default:
			l.addPart(p.ent, p.path)
		}
	}
	l.pending = nil
}

func (c *checker) beginStmt() {
	c.consumed.depth++
}

func (c *checker) endStmt() {
	if c.consumed.depth--; c.consumed.depth == 0 {
		c.consumed.commit()
	}
}
