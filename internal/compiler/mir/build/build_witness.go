package build

import (
	"fmt"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// witnessLocal reads a hidden witness local (a function value) as a
// temporary, so the call consumes a copy and the local stays live.
func (b *builder) witnessLocal(ent sem.EntityID) midir.LocalID {
	l := b.local(ent)
	return b.emit(midir.Inst{Op: midir.OpCopy, Dst: b.temp(b.fn.Locals[l].Type), Args: []midir.LocalID{l}})
}

// witnessArgs lowers the witness values a call passes after its ordinary
// arguments: the caller's own slot, a function item, or a function with
// its own witnesses bound.
func (b *builder) witnessArgs(passes []sem.WitnessArg, n syntax.NodeID) []midir.LocalID {
	var out []midir.LocalID
	for _, w := range passes {
		out = append(out, b.witnessArg(w, n))
	}
	return out
}

func (b *builder) witnessArg(w sem.WitnessArg, n syntax.NodeID) midir.LocalID {
	switch w.Kind {
	case sem.WitnessLocal:
		return b.witnessLocal(w.Ent)
	case sem.WitnessFn:
		// Bound with nothing: the bind adapter (or the std bind of a
		// primitive) also settles a borrowed receiver the way a direct
		// method call would.
		if info := b.r.Fn(w.Ent); len(info.Witnesses)+len(info.ConstParams) > 0 {
			// A raw item keeps its hidden params open, so the requirement
			// call can supply its own const args.
			return b.emit(midir.Inst{Op: midir.OpFnItem, Dst: b.temp(info.Sig), Ent: w.Ent, Node: n})
		}
		return b.emit(midir.Inst{Op: midir.OpBind, Dst: b.temp(b.r.Fn(w.Ent).Sig), Ent: w.Ent, Node: n})
	case sem.WitnessBind:
		caps := append(b.witnessArgs(w.Args, n), b.constArgs(w.ConstArgs, n)...)
		return b.emit(midir.Inst{Op: midir.OpBind, Dst: b.temp(b.r.Fn(w.Ent).Sig), Ent: w.Ent, Args: caps, Node: n})
	}
	// Zero Kind means sem accepted a call it could not build a witness
	// for: a checker bug, not a user error, so this must never reach a
	// baked runtime OpPanic.
	panic(fmt.Sprintf("mir: %s: empty WitnessArg for an accepted call", b.fn.Name))
}
