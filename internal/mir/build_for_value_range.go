package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// valueRangeCursor walks a Range held in a value, whose upper bound is open or closed only at run time.
type valueRangeCursor struct {
	b            *builder
	cur, hi, inc LocalID
	typ          sem.TypeID
}

func (b *builder) valueRangeCursor(e *hir.Expr) *valueRangeCursor {
	t := e.Pat.Type
	rng := b.expr(e.Cond)
	if b.fn.Locals[rng].Ent == 0 {
		b.own(rng)
	}
	field := func(i int, ft sem.TypeID) LocalID {
		f := b.emit(Inst{Op: OpField, Dst: b.temp(ft), Args: []LocalID{rng}, Index: i, Node: e.Node})
		return b.own(b.emit(Inst{Op: OpCopy, Dst: b.temp(ft), Args: []LocalID{f}, Node: e.Node}))
	}
	return &valueRangeCursor{b: b, typ: t, cur: field(0, t), hi: field(1, t), inc: field(2, sem.TyBool)}
}

func (c *valueRangeCursor) test(n syntax.NodeID) LocalID {
	b := c.b
	res := b.temp(sem.TyBool)
	closed, open, join := b.newBlockAt(), b.newBlockAt(), b.newBlockAt()
	b.term(Term{Op: TermBranch, Args: []LocalID{c.inc}, Targets: []BlockID{closed, open}})
	for i, op := range []string{"<=", "<"} {
		b.cur = []BlockID{closed, open}[i]
		b.assign(res, b.emit(Inst{Op: OpBinary, Dst: b.temp(sem.TyBool), Str: op, Args: []LocalID{c.cur, c.hi}, Node: n}))
		b.jump(join)
	}
	b.cur = join
	return res
}

func (c *valueRangeCursor) elem(n syntax.NodeID) LocalID {
	return c.b.emit(Inst{Op: OpCopy, Dst: c.b.temp(c.typ), Args: []LocalID{c.cur}, Node: n, Share: true})
}

// A body only runs while cur is in range, so cur == hi can only be the last element of a closed range.
func (c *valueRangeCursor) advance(n syntax.NodeID, _, exit BlockID) {
	b := c.b
	last := b.emit(Inst{Op: OpBinary, Dst: b.temp(sem.TyBool), Str: "==", Args: []LocalID{c.cur, c.hi}, Node: n})
	step := b.newBlockAt()
	b.term(Term{Op: TermBranch, Args: []LocalID{last}, Targets: []BlockID{exit, step}})
	b.cur = step
	b.stepCounter(c.cur, c.typ, n)
}
