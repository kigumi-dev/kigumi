package build

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

type forCursor interface {
	test(n syntax.NodeID) midir.LocalID
	elem(n syntax.NodeID) midir.LocalID
	advance(n syntax.NodeID, cond, exit midir.BlockID)
}

func (b *builder) forCursorFor(e *hir.Expr) forCursor {
	if c := b.rangeCursor(e); c != nil {
		return c
	}
	if rng := b.r.LangItem("Range"); rng != 0 {
		if n := b.r.Types.Node(e.Cond.Type); n.Kind == sem.KNamed && n.Ent == rng {
			return b.valueRangeCursor(e)
		}
	}
	return b.indexCursor(e)
}

type indexCursor struct {
	b                 *builder
	iter, idx, length midir.LocalID
	at                string
	typ               sem.TypeID
}

func (b *builder) indexCursor(e *hir.Expr) *indexCursor {
	pattern, head, n := e.Pat, e.Cond, e.Node
	// Only a plain owned head is moved by the loop; a borrowed head and a lent `self` are not, as in declare().
	refHead := b.r.Types.Kind(head.Type) == sem.KRef
	c := &indexCursor{b: b, typ: pattern.Type}
	if lid, ok := b.locals[head.Ent]; !refHead && head.Kind == hir.Local && ok && !b.fn.Locals[lid].Borrowed {
		c.iter = b.exprMoved(head)
		b.markRetain(c.iter)
	} else {
		c.iter = b.expr(head)
	}
	if b.fn.Locals[c.iter].Ent == 0 {
		if refHead {
			b.ownBorrowedSource(c.iter)
		} else {
			b.own(c.iter)
		}
	}
	c.idx = b.own(b.emit(midir.Inst{Op: midir.OpConst, Dst: b.temp(sem.TyUsize), Lit: sem.Literal{Kind: sem.LitInt}, Type: sem.TyUsize, Str: "0", Node: n}))
	c.length = b.own(b.emit(midir.Inst{Op: midir.OpBuiltin, Dst: b.temp(sem.TyUsize), Str: "iter.len", Args: []midir.LocalID{c.iter}, Node: n}))
	// Copy keeps iter.at (std loops like Array.reverse re-read by index); a borrowed head binds through
	// iter.at_ref; an owned element moves once via iter.at_move.
	c.at = "iter.at"
	switch {
	case b.r.IsCopy(pattern.Type):
	case refHead:
		c.at = "iter.at_ref"
	default:
		c.at = "iter.at_move"
	}
	return c
}

func (c *indexCursor) test(n syntax.NodeID) midir.LocalID {
	return c.b.emit(midir.Inst{Op: midir.OpBinary, Dst: c.b.temp(sem.TyBool), Str: "<", Args: []midir.LocalID{c.idx, c.length}, Node: n})
}

func (c *indexCursor) elem(n syntax.NodeID) midir.LocalID {
	return c.b.emit(midir.Inst{Op: midir.OpBuiltin, Dst: c.b.temp(c.typ), Str: c.at, Args: []midir.LocalID{c.iter, c.idx}, Node: n})
}

func (c *indexCursor) advance(n syntax.NodeID, _, _ midir.BlockID) {
	c.b.stepCounter(c.idx, sem.TyUsize, n)
}

func (b *builder) stepCounter(counter midir.LocalID, t sem.TypeID, n syntax.NodeID) {
	one := b.emit(midir.Inst{Op: midir.OpConst, Dst: b.temp(t), Lit: sem.Literal{Kind: sem.LitInt}, Type: t, Str: "1", Node: n})
	sum := b.emit(midir.Inst{Op: midir.OpBinary, Dst: b.temp(t), Str: "+", Args: []midir.LocalID{counter, one}, Node: n, NoTrap: true})
	b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{counter}, Node: n})
	b.emit(midir.Inst{Op: midir.OpCopy, Dst: counter, Args: []midir.LocalID{sum}, Node: n})
}
