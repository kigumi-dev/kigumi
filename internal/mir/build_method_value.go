package mir

import (
	"kigumi/internal/hir"
	"kigumi/internal/sem"
)

func (b *builder) methodValue(e *hir.Expr, recv LocalID, t sem.TypeID) LocalID {
	if b.r.Entity(b.r.Entity(e.Ent).Parent).Kind == sem.EntInterface {
		return b.requirementValue(e, recv, t)
	}
	if b.r.Fn(e.Ent).Recv == sem.RecvMut {
		recv = b.emit(Inst{Op: OpNewCell, Dst: b.temp(b.fn.Locals[recv].Type), Args: []LocalID{recv}, Node: e.Node})
	}
	if len(e.Passes) > 0 {
		return b.appliedValue(e, recv, t)
	}
	async := b.r.Fn(e.Ent).Declared&sem.EffAsync != 0
	return b.emit(Inst{Op: OpClosure, Dst: b.temp(t), Ent: e.Ent, Args: []LocalID{recv}, Str: "method", Async: async, Node: e.Node})
}

func (b *builder) requirementValue(e *hir.Expr, recv LocalID, t sem.TypeID) LocalID {
	if e.Witness == 0 {
		return b.emit(Inst{Op: OpClosure, Dst: b.temp(t), Ent: e.Ent, Args: []LocalID{recv}, Str: "method", Node: e.Node})
	}
	return b.emit(Inst{Op: OpClosure, Dst: b.temp(t), Ent: e.Ent, Args: []LocalID{b.witnessLocal(e.Witness), recv}, Str: "witness", Node: e.Node})
}

// A method value over a constrained generic holds the method bound to its witnesses and the receiver, applied at each call.
func (b *builder) appliedValue(e *hir.Expr, recv LocalID, t sem.TypeID) LocalID {
	async := b.r.Fn(e.Ent).Declared&sem.EffAsync != 0
	bound := b.emit(Inst{Op: OpBind, Dst: b.temp(b.r.Fn(e.Ent).Sig), Ent: e.Ent, Args: b.witnessArgs(e.Passes, e.Node), Async: async, Node: e.Node})
	return b.emit(Inst{Op: OpClosure, Dst: b.temp(t), Ent: e.Ent, Args: []LocalID{bound, recv}, Str: "applied", Async: async, Node: e.Node})
}

// A closure-typed method value owns its receiver: a binding or temporary moves in, a field moves out of its owner.
func (b *builder) methodReceiver(e *hir.Expr, callable sem.TypeID) LocalID {
	if b.r.Types.Kind(callable) != sem.KClosure {
		return b.expr(e.Args[0])
	}
	r := b.unparen(e.Args[0])
	if r.Kind == hir.Field {
		base := b.expr(r.Args[0])
		return b.emit(Inst{Op: OpFieldMove, Dst: b.temp(r.Type), Args: []LocalID{base}, Index: r.Index, Node: r.Node})
	}
	return b.exprMoved(r)
}
