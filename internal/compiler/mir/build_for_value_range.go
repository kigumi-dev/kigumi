package mir

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// valueRangeCursor walks a Range held in a value, whose upper bound is open or closed only at run time.
type valueRangeCursor struct {
	b            *builder
	cur, hi, inc midir.LocalID
	typ          sem.TypeID
}

func (b *builder) valueRangeCursor(e *hir.Expr) *valueRangeCursor {
	t := e.Pat.Type
	rng := b.expr(e.Cond)
	if b.fn.Locals[rng].Ent == 0 {
		b.own(rng)
	}
	field := func(i int, ft sem.TypeID) midir.LocalID {
		f := b.emit(midir.Inst{Op: midir.OpField, Dst: b.temp(ft), Args: []midir.LocalID{rng}, Index: i, Node: e.Node})
		return b.own(b.emit(midir.Inst{Op: midir.OpCopy, Dst: b.temp(ft), Args: []midir.LocalID{f}, Node: e.Node}))
	}
	return &valueRangeCursor{b: b, typ: t, cur: field(0, t), hi: field(1, t), inc: field(2, sem.TyBool)}
}

func (c *valueRangeCursor) test(n syntax.NodeID) midir.LocalID {
	b := c.b
	res := b.temp(sem.TyBool)
	closed, open, join := b.newBlockAt(), b.newBlockAt(), b.newBlockAt()
	b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{c.inc}, Targets: []midir.BlockID{closed, open}})
	for i, op := range []string{"<=", "<"} {
		b.cur = []midir.BlockID{closed, open}[i]
		b.assign(res, b.emit(midir.Inst{Op: midir.OpBinary, Dst: b.temp(sem.TyBool), Str: op, Args: []midir.LocalID{c.cur, c.hi}, Node: n}))
		b.jump(join)
	}
	b.cur = join
	return res
}

func (c *valueRangeCursor) elem(n syntax.NodeID) midir.LocalID {
	return c.b.emit(midir.Inst{Op: midir.OpCopy, Dst: c.b.temp(c.typ), Args: []midir.LocalID{c.cur}, Node: n, Share: true})
}

// A body only runs while cur is in range, so cur == hi can only be the last element of a closed range.
func (c *valueRangeCursor) advance(n syntax.NodeID, _, exit midir.BlockID) {
	b := c.b
	last := b.emit(midir.Inst{Op: midir.OpBinary, Dst: b.temp(sem.TyBool), Str: "==", Args: []midir.LocalID{c.cur, c.hi}, Node: n})
	step := b.newBlockAt()
	b.term(midir.Term{Op: midir.TermBranch, Args: []midir.LocalID{last}, Targets: []midir.BlockID{exit, step}})
	b.cur = step
	b.stepCounter(c.cur, c.typ, n)
}
