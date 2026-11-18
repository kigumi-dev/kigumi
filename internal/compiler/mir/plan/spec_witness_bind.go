package plan

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// witnessSite is the bind or item that builds a witness a call hands over, with the instantiation the call
// binds the witness's type parameter to.
type witnessSite struct {
	blk, idx int
	fn       sem.EntityID
	t        sem.TypeID
}

type defPos struct{ blk, idx int }

// witnessSites lists the witnesses of call in whose body cannot run erased.
func (s *specializer) witnessSites(f *midir.Func, in midir.Inst, subst func(sem.TypeID) sem.TypeID, need map[sem.EntityID]map[sem.EntityID]bool) []witnessSite {
	r := s.p.R
	gen := r.GenericOf(in.Ent)
	if in.Op != midir.OpCall && in.Op != midir.OpBind || gen == 0 || r.Entity(gen).Kind != sem.EntFn {
		return nil
	}
	slots := r.Fn(gen).Witnesses
	decl := s.p.declaredArgs(gen)
	if in.Op == midir.OpBind {
		decl = nil
	}
	if len(slots) == 0 || in.Op == midir.OpCall && decl == nil || len(in.Args) < len(decl)+len(slots) {
		return nil
	}
	call := in
	call.Ent = gen
	var args []sem.TypeID
	ok := false
	if in.Op == midir.OpBind {
		args, ok = s.p.valueArgs(f, call, subst)
	} else {
		args, ok = s.p.bindTypeArgsVia(f, call, subst, bindForced)
	}
	if !ok {
		return nil
	}
	var out []witnessSite
	for k, slot := range slots {
		pos, ok := s.defOf(f, in.Args[len(decl)+k])
		if !ok {
			continue
		}
		def := f.Blocks[pos.blk].Insts[pos.idx]
		if def.Op != midir.OpBind && def.Op != midir.OpFnItem || len(need[def.Ent]) == 0 {
			continue
		}
		if i := slices.Index(r.Fn(gen).TypeParams, slot.Param); i >= 0 && args[i] != 0 {
			out = append(out, witnessSite{pos.blk, pos.idx, def.Ent, args[i]})
		}
	}
	return out
}

// specializeWitnesses makes each such witness of the call at (bid, idx) name the body for its instantiation.
func (s *specializer) specializeWitnesses(f *midir.Func, ctx *spec, bid, idx int) {
	r := s.p.R
	for _, w := range s.witnessSites(f, f.Blocks[bid].Insts[idx], func(t sem.TypeID) sem.TypeID { return s.inCtx(ctx, t) }, s.shapeParams()) {
		args, ok := s.receiverShapeArgs(w.fn, w.t)
		if !ok {
			continue
		}
		in := midir.Inst{Op: midir.OpCall, Ent: w.fn, Node: f.Blocks[w.blk].Insts[w.idx].Node}
		sp := s.get(f, ctx, in, r.EraseLifetimes(args), nil)
		if sp == nil {
			continue
		}
		s.rename(f, w.blk, w.idx, sp)
	}
}

// receiverShapeArgs binds the shape parameters of method fn from t, the instantiation of its receiver.
func (s *specializer) receiverShapeArgs(fn sem.EntityID, t sem.TypeID) ([]sem.TypeID, bool) {
	r := s.p.R
	bound, ok := r.ReceiverArgs(fn, t)
	if !ok {
		return nil, false
	}
	params := r.Fn(fn).TypeParams
	args := make([]sem.TypeID, len(params))
	for i, p := range params {
		if !r.IsLifetimeParam(p) {
			args[i] = bound[p]
		}
	}
	return s.shapeArgsOnly(fn, args)
}

// defOf finds the one instruction defining l; a cached position a rewrite moved is looked up again.
func (s *specializer) defOf(f *midir.Func, l midir.LocalID) (defPos, bool) {
	defs, cached := s.defs[f]
	if !cached {
		defs = s.singleDefPositions(f)
	}
	pos, ok := defs[l]
	if ok && !defines(f, pos, l) && cached {
		pos, ok = s.singleDefPositions(f)[l]
	}
	return pos, ok && defines(f, pos, l)
}

func defines(f *midir.Func, pos defPos, l midir.LocalID) bool {
	if pos.blk >= len(f.Blocks) || pos.idx >= len(f.Blocks[pos.blk].Insts) {
		return false
	}
	in := f.Blocks[pos.blk].Insts[pos.idx]
	return in.Op.Yields() && in.Dst == l
}

func (s *specializer) singleDefPositions(f *midir.Func) map[midir.LocalID]defPos {
	count := map[midir.LocalID]int{}
	out := map[midir.LocalID]defPos{}
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op.Yields() {
				count[in.Dst]++
				out[in.Dst] = defPos{bid, i}
			}
		}
	}
	for l, n := range count {
		if n != 1 {
			delete(out, l)
		}
	}
	s.defs[f] = out
	return out
}
