package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

type rangeCursor struct {
	b         *builder
	cur, hi   LocalID
	typ       sem.TypeID
	inclusive bool
}

func (b *builder) rangeCursor(e *hir.Expr) *rangeCursor {
	head := e.Cond
	if head.Kind != hir.Binary || head.Ent != 0 || len(head.Args) != 2 || (head.Name != ".." && head.Name != "..=") {
		return nil
	}
	t := e.Pat.Type
	if !b.r.Types.IsInteger(t) || sem.WideScalar(t) || head.Args[0].Type != t || head.Args[1].Type != t {
		return nil
	}
	c := &rangeCursor{b: b, typ: t, inclusive: head.Name == "..="}
	lo := b.expr(head.Args[0])
	hi := b.expr(head.Args[1])
	c.cur = b.own(b.emit(Inst{Op: OpCopy, Dst: b.temp(t), Args: []LocalID{lo}, Node: e.Node}))
	c.hi = b.own(b.emit(Inst{Op: OpCopy, Dst: b.temp(t), Args: []LocalID{hi}, Node: e.Node}))
	return c
}

func (c *rangeCursor) test(n syntax.NodeID) LocalID {
	op := "<"
	if c.inclusive {
		op = "<="
	}
	return c.b.emit(Inst{Op: OpBinary, Dst: c.b.temp(sem.TyBool), Str: op, Args: []LocalID{c.cur, c.hi}, Node: n})
}

// Share keeps the binding's drop from releasing the counter itself.
func (c *rangeCursor) elem(n syntax.NodeID) LocalID {
	return c.b.emit(Inst{Op: OpCopy, Dst: c.b.temp(c.typ), Args: []LocalID{c.cur}, Node: n, Share: true})
}

// An inclusive range stops at hi before stepping, so a hi at the type's maximum never overflows.
func (c *rangeCursor) advance(n syntax.NodeID, _, exit BlockID) {
	b := c.b
	if c.inclusive {
		last := b.emit(Inst{Op: OpBinary, Dst: b.temp(sem.TyBool), Str: "==", Args: []LocalID{c.cur, c.hi}, Node: n})
		step := b.newBlockAt()
		b.term(Term{Op: TermBranch, Args: []LocalID{last}, Targets: []BlockID{exit, step}})
		b.cur = step
	}
	b.stepCounter(c.cur, c.typ, n)
}
