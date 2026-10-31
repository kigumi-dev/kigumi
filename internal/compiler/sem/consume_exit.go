package sem

import "kigumi/internal/compiler/syntax"

// exitTake collects what an exit's own value moves on every path that reaches the exit.
type exitTake struct {
	moved        []movedPlace
	branch, loop int
	outer        *exitTake
}

func (l *consumeLog) declare(id EntityID) {
	if len(l.exits) == 0 {
		return
	}
	if l.born == nil {
		l.born = map[EntityID]int{}
	}
	l.born[id] = len(l.exits)
}

// exitedSince reports whether an exit checked from index from on leaves m in place, to be dropped there.
func (l *consumeLog) exitedSince(m movedPlace, from int) bool {
	for _, spared := range l.exits[from:] {
		if !spares(spared, m) {
			return true
		}
	}
	return false
}

func spares(moved []movedPlace, m movedPlace) bool {
	for _, s := range moved {
		if s.ent == m.ent && isFieldPrefix(s.path, m.path) {
			return true
		}
	}
	return false
}

func (c *checker) beginExit() *exitTake {
	saved := c.consumed.taking
	c.consumed.taking = &exitTake{branch: c.branch, loop: c.loop, outer: saved}
	return saved
}

func (c *checker) endExit(saved *exitTake) {
	c.consumed.exits = append(c.consumed.exits, c.consumed.taking.moved)
	c.consumed.taking = saved
}

func (c *checker) noteExitTake(m movedPlace) {
	for t := c.consumed.taking; t != nil; t = t.outer {
		if t.branch == c.branch && t.loop == c.loop {
			t.moved = append(t.moved, m)
		}
	}
}

// movedOut is the binding or field path under one that consuming a moves out.
func (c *checker) movedOut(a syntax.NodeID) (movedPlace, bool) {
	if c.t.Kind(a) == syntax.Ident {
		ent := c.info.Uses[a]
		if ent == 0 {
			return movedPlace{}, false
		}
		k := c.r.Entities[ent].Kind
		return movedPlace{ent: ent}, k == EntLocal || k == EntParam
	}
	p, ok := c.consumedPlace(a)
	if !ok || len(p.fields) == 0 {
		return movedPlace{}, false
	}
	k := c.r.Entities[p.ent].Kind
	return movedPlace{ent: p.ent, path: p.fields}, k == EntLocal || k == EntParam
}
