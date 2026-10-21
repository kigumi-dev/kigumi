package sem

import (
	"slices"
	"strings"

	"kigumi/internal/syntax"
)

type scalarCopy struct {
	scrutinee string
	fields    []string
}

func (c *checker) noteCopyBinding(ent EntityID, tail ...EntityID) {
	if !c.patBorrowMut {
		return
	}
	if _, ok := c.r.ScalarRefElem(c.vars.resolve(c.r.Entities[ent].Type)); !ok {
		return
	}
	var names []string
	for _, f := range append(slices.Clone(c.patMoved.path), tail...) {
		if f != 0 {
			names = append(names, c.r.Entities[f].Name)
		}
	}
	if len(names) != c.held.patDepth-1+len(tail) {
		names = nil
	}
	if c.held.scalarCopy == nil {
		c.held.scalarCopy = map[EntityID]scalarCopy{}
	}
	c.held.scalarCopy[ent] = scalarCopy{fields: names}
}

// A `for` head has no scrutinee expression and already rejects these bindings.
func (c *checker) noteCopyScrutinee(scrutinee syntax.NodeID, bound []EntityID) {
	if scrutinee == 0 {
		for _, ent := range bound {
			delete(c.held.scalarCopy, ent)
		}
		return
	}
	text := c.placeText(scrutinee)
	for _, ent := range bound {
		if sc, ok := c.held.scalarCopy[ent]; ok {
			sc.scrutinee = text
			c.held.scalarCopy[ent] = sc
		}
	}
}

func (c *checker) placeText(n syntax.NodeID) string {
	n = c.t.Unparen(n)
	if c.t.Kind(n) == syntax.BorrowExpr {
		n = c.t.View(n).AsBorrow().Operand().ID()
	}
	sp := c.t.Span(n)
	text := string(c.t.File.Src[sp.Start:sp.End])
	for _, r := range text {
		if r != '.' && r != '_' && (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return ""
		}
	}
	return text
}

func (sc scalarCopy) place() string {
	if sc.scrutinee != "" && len(sc.fields) > 0 {
		return "`" + sc.scrutinee + "." + strings.Join(sc.fields, ".") + "`"
	}
	return ""
}

func (c *checker) rejectScalarCopyWrite(lhs syntax.NodeID) {
	ent, ok := c.identPlaceEnt(lhs)
	if !ok {
		return
	}
	sc, ok := c.held.scalarCopy[ent]
	if !ok {
		return
	}
	place := sc.place()
	if place == "" {
		place = "the matched place"
	}
	c.errAt(lhs, cScalarCopyWrite, c.r.Entities[ent].Name, place)
}

func (c *checker) rejectScalarCopyArg(arg syntax.NodeID, param TypeID) {
	if _, ok := c.r.ScalarRefElem(c.vars.resolve(param)); !ok {
		return
	}
	ent, ok := c.identPlaceEnt(c.t.Unparen(arg))
	if !ok {
		return
	}
	sc, ok := c.held.scalarCopy[ent]
	if !ok {
		return
	}
	place := "a `&mut` borrow of the matched place"
	if p := sc.place(); p != "" {
		place = "`&mut " + p[1:]
	}
	c.errAt(arg, cScalarCopyArg, c.r.Entities[ent].Name, place)
}
