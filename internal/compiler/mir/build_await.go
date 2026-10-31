package mir

import (
	"kigumi/internal/compiler/hir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// await consumes the future, so it moves like Fail; mode "settle" discards the result and continues past a cancelled Task.
func (b *builder) await(e *hir.Expr, n syntax.NodeID, t sem.TypeID, mode string) LocalID {
	fut := b.exprMoved(e.Args[0])
	b.fn.Suspends++
	v := b.emit(Inst{Op: OpAwait, Dst: b.temp(t), Args: []LocalID{fut}, Index: b.fn.Suspends, Str: mode, Node: n})
	b.cancelEdge(n)
	return v
}

// closeTaskScope marks where an async body starts to leave; the runtime closes the task's scope there when the body is the task's own.
func (b *builder) closeTaskScope(n syntax.NodeID) {
	if b.fn.Async {
		b.emit(Inst{Op: OpBuiltin, Dst: b.temp(sem.TyUnit), Str: "task.close", Node: n})
	}
}

// settles lowers an `await` of a Task whose result is discarded; it reports false for anything else.
func (b *builder) settles(e *hir.Expr) bool {
	x := b.unparen(e)
	if x.Kind != hir.Await || !b.r.IsTaskType(x.Args[0].Type) {
		return false
	}
	b.await(x, x.Node, sem.TyUnit, "settle")
	return true
}
