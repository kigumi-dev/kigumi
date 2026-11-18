package plan

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// witnessTypes maps each witness value a call in f hands over to the instantiation the call binds its type
// parameter to: the receiver a method witness runs for.
func (ps *envPass) witnessTypes(f *midir.Func) map[midir.LocalID]sem.TypeID {
	if m, ok := ps.witnesses[f]; ok {
		return m
	}
	r := ps.p.R
	out := map[midir.LocalID]sem.TypeID{}
	ps.witnesses[f] = out
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			gen := r.GenericOf(in.Ent)
			if in.Op != midir.OpCall && in.Op != midir.OpBind || gen == 0 || r.Entity(gen).Kind != sem.EntFn || len(r.Fn(gen).Witnesses) == 0 {
				continue
			}
			first := 0
			call := in
			call.Ent = gen
			var args []sem.TypeID
			ok := false
			if in.Op == midir.OpBind {
				args, ok = ps.p.valueArgs(f, call, ps.substOf(f))
			} else {
				first = len(ps.p.declaredArgs(gen))
				args, ok = ps.p.bindTypeArgsVia(f, call, ps.substOf(f), bindRuntime)
			}
			if !ok {
				continue
			}
			for k, slot := range r.Fn(gen).Witnesses {
				i := slices.Index(r.Fn(gen).TypeParams, slot.Param)
				if first+k < len(in.Args) && i >= 0 && args[i] != 0 {
					out[in.Args[first+k]] = args[i]
				}
			}
		}
	}
	return out
}

// receiverArgs binds method fn's type parameters from t, the instantiation of its receiver, in TypeParams order.
func (ps *envPass) receiverArgs(fn sem.EntityID, t sem.TypeID) ([]sem.TypeID, bool) {
	r := ps.p.R
	bound, ok := r.ReceiverArgs(fn, t)
	if !ok {
		return nil, false
	}
	tps := r.Fn(fn).TypeParams
	out := make([]sem.TypeID, len(tps))
	for i, p := range tps {
		out[i] = bound[p]
		if out[i] == 0 {
			out[i] = r.Types.Param(p)
		}
	}
	return out, true
}
