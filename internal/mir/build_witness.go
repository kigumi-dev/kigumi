package mir

import (
	"fmt"

	"kigumi/internal/hir"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// witnessLocal reads a hidden witness local (a function value) as a
// temporary, so the call consumes a copy and the local stays live.
func (b *builder) witnessLocal(ent sem.EntityID) LocalID {
	l := b.local(ent)
	return b.emit(Inst{Op: OpCopy, Dst: b.temp(b.fn.Locals[l].Type), Args: []LocalID{l}})
}

// witnessArgs lowers the witness values a call passes after its ordinary
// arguments: the caller's own slot, a function item, or a function with
// its own witnesses bound.
func (b *builder) witnessArgs(passes []sem.WitnessArg, n syntax.NodeID) []LocalID {
	var out []LocalID
	for _, w := range passes {
		out = append(out, b.witnessArg(w, n))
	}
	return out
}

func (b *builder) witnessArg(w sem.WitnessArg, n syntax.NodeID) LocalID {
	switch w.Kind {
	case sem.WitnessLocal:
		return b.witnessLocal(w.Ent)
	case sem.WitnessFn:
		// Bound with nothing: the bind adapter (or the std bind of a
		// primitive) also settles a borrowed receiver the way a direct
		// method call would.
		return b.emit(Inst{Op: OpBind, Dst: b.temp(b.r.Fn(w.Ent).Sig), Ent: w.Ent, Node: n})
	case sem.WitnessBind:
		caps := append(b.witnessArgs(w.Args, n), b.constArgs(w.ConstArgs, n)...)
		return b.emit(Inst{Op: OpBind, Dst: b.temp(b.r.Fn(w.Ent).Sig), Ent: w.Ent, Args: caps, Node: n})
	}
	// Zero Kind means sem accepted a call it could not build a witness
	// for: a checker bug, not a user error, so this must never reach a
	// baked runtime OpPanic.
	panic(fmt.Sprintf("mir: %s: empty WitnessArg for an accepted call", b.fn.Name))
}

// LentMask says which arguments a std primitive used as a witness borrows
// rather than consumes (bit i for argument i): a `self` / `mut self`
// receiver and every `&T` parameter. rt_callv releases the others.
func LentMask(r *sem.Result, fn sem.EntityID) int {
	info := r.Fn(fn)
	mask, i := 0, 0
	if info.Recv != sem.RecvNone {
		if info.Recv != sem.RecvMove {
			mask |= 1
		}
		i = 1
	}
	for _, t := range r.Types.Node(info.Sig).Args {
		if r.Types.Kind(t) == sem.KRef {
			mask |= 1 << i
		}
		i++
	}
	return mask
}

func (b *builder) requirementValue(e *hir.Expr, recv LocalID) LocalID {
	if e.Witness == 0 {
		return b.emit(Inst{Op: OpClosure, Dst: b.temp(e.Type), Ent: e.Ent, Args: []LocalID{recv}, Str: "method", Node: e.Node})
	}
	return b.emit(Inst{Op: OpClosure, Dst: b.temp(e.Type), Ent: e.Ent, Args: []LocalID{b.witnessLocal(e.Witness), recv}, Str: "witness", Node: e.Node})
}

// A method value over a constrained generic holds the method bound to its witnesses and the receiver, applied at each call.
func (b *builder) appliedValue(e *hir.Expr, recv LocalID) LocalID {
	async := b.r.Fn(e.Ent).Declared&sem.EffAsync != 0
	bound := b.emit(Inst{Op: OpBind, Dst: b.temp(b.r.Fn(e.Ent).Sig), Ent: e.Ent, Args: b.witnessArgs(e.Passes, e.Node), Async: async, Node: e.Node})
	return b.emit(Inst{Op: OpClosure, Dst: b.temp(e.Type), Ent: e.Ent, Args: []LocalID{bound, recv}, Str: "applied", Async: async, Node: e.Node})
}

// A closure-typed method value owns its receiver, so the binding moves into it.
func (b *builder) methodReceiver(e *hir.Expr) LocalID {
	if b.r.Types.Kind(e.Type) == sem.KClosure {
		return b.exprMoved(e.Args[0])
	}
	return b.expr(e.Args[0])
}
