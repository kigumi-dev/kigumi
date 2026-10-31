package mir

import (
	"slices"

	"kigumi/internal/compiler/hir"
	"kigumi/internal/compiler/sem"
)

// The builtin names the static in Ent and the memory order in Index.
func (b *builder) atomicStatic(e *hir.Expr) (LocalID, bool) {
	op, ok := b.r.MemOpOf(e.Ent)
	if ok && op.Addr {
		return b.volatileAt(e, op), true
	}
	if !ok || !op.Atomic || op.Cell || len(e.Args) < 2 {
		return 0, false
	}
	place := e.Args[0]
	for place.Kind == hir.Wrap && len(place.Args) == 1 {
		place = place.Args[0]
	}
	if place.Kind != hir.Borrow || len(place.Args) != 1 || place.Args[0].Kind != hir.Global {
		return 0, false
	}
	order := e.Args[len(e.Args)-1]
	for order.Kind == hir.Wrap && len(order.Args) == 1 {
		order = order.Args[0]
	}
	if order.Kind != hir.Variant {
		return 0, false
	}
	name, ok := atomicBuiltins[op.Op]
	if !ok {
		return 0, false
	}
	args := b.args(e, e.Args[1:len(e.Args)-1], nil, true, false, 0)
	return b.emit(Inst{Op: OpBuiltin, Dst: b.temp(e.Type), Str: name, Ent: place.Args[0].Ent, Index: slices.Index(sem.AtomicOrders, b.r.Entity(order.Ent).Name), Args: args, Node: e.Node}), true
}

func (b *builder) volatileAt(e *hir.Expr, op sem.MemOp) LocalID {
	name := "volatile.read"
	if op.Op == "volatileWriteAt" {
		name = "volatile.write"
	}
	args := b.args(e, e.Args, nil, true, false, 0)
	return b.emit(Inst{Op: OpBuiltin, Dst: b.temp(e.Type), Str: name, Args: args, Node: e.Node})
}

var atomicBuiltins = map[string]string{
	"load": "atomic.load", "store": "atomic.store", "exchange": "atomic.exchange", "compareExchange": "atomic.compareExchange",
	"add": "atomic.add", "sub": "atomic.sub", "and": "atomic.and", "or": "atomic.or", "xor": "atomic.xor",
}
