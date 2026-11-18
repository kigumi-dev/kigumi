package plan

import (
	"kigumi/internal/compiler/mir/instance"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// shapeParams are, per body, the SIMD lane shapes it or a method-table or drop body its boxes reach cannot run
// without; every call instantiates them regardless of the budget. A runtime type never needs that.
func (s *specializer) shapeParams() map[sem.EntityID]map[sem.EntityID]bool {
	if s.shapes == nil {
		s.shapes = s.paramsNeeded()
	}
	return s.shapes
}

func (s *specializer) paramsNeeded() map[sem.EntityID]map[sem.EntityID]bool {
	r := s.p.R
	need := map[sem.EntityID]map[sem.EntityID]bool{}
	for _, f := range s.p.Funcs {
		if r.Specializable(f.Ent) || s.simdAsync(f.Ent) || r.IsDropFn(f.Ent) || f.Ent != 0 && r.Entity(f.Ent).Kind == sem.EntClosure {
			need[f.Ent] = map[sem.EntityID]bool{}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, f := range s.p.Funcs {
			params := need[f.Ent]
			if params == nil {
				continue
			}
			add := func(t sem.TypeID) {
				for _, p := range instance.Params(r, t) {
					if !params[p] {
						params[p], changed = true, true
					}
				}
			}
			for _, blk := range f.Blocks {
				for _, in := range blk.Insts {
					s.instShapes(f, in, need, add)
				}
			}
		}
	}
	return need
}

func (s *specializer) instShapes(f *midir.Func, in midir.Inst, need map[sem.EntityID]map[sem.EntityID]bool, add func(sem.TypeID)) {
	r := s.p.R
	if in.Op == midir.OpClosure && r.Entity(in.Ent).Kind == sem.EntClosure {
		for p := range need[in.Ent] {
			add(r.Types.Param(p))
		}
		return
	}
	if s.p.SimdCall(in) {
		add(f.Locals[in.Dst].Type)
		for _, a := range in.Args {
			add(f.Locals[a].Type)
		}
		return
	}
	for _, t := range s.descriptorTypes(f, in, need) {
		add(t)
	}
	for _, w := range s.witnessSites(f, in, func(t sem.TypeID) sem.TypeID { return t }, need) {
		add(w.t)
	}
	callee := need[in.Ent]
	if len(callee) == 0 {
		return
	}
	args, ok := s.p.valueArgs(f, in, func(t sem.TypeID) sem.TypeID { return t })
	if !ok {
		args, ok = s.p.bindTypeArgsVia(f, s.callBinding(f, in), func(t sem.TypeID) sem.TypeID { return t }, bindForced)
	}
	if !ok {
		return
	}
	for i, p := range r.Fn(in.Ent).TypeParams {
		if callee[p] {
			add(args[i])
		}
	}
}

// descriptorTypes are the types whose descriptors in builds for a method-table or drop body that needs a lane
// shape: a box's type, or the type arguments of a record its drop needs.
func (s *specializer) descriptorTypes(f *midir.Func, in midir.Inst, need map[sem.EntityID]map[sem.EntityID]bool) []sem.TypeID {
	r := s.p.R
	switch in.Op {
	case midir.OpBox, midir.OpLendBox:
		iface := f.Locals[in.Dst].Type
		if in.Op == midir.OpLendBox {
			iface = r.Types.Node(iface).Elem
		}
		for _, w := range r.Witnesses(in.Type, iface, r.Entity(f.Ent).Pkg) {
			if len(need[w]) > 0 {
				return []sem.TypeID{in.Type}
			}
		}
	case midir.OpRecord:
		return s.dropNeeds(in.Ent, f.Locals[in.Dst].Type, need)
	}
	return nil
}

// dropNeeds are the type arguments of t, an instantiation of ent, that ent's drop needs as types.
func (s *specializer) dropNeeds(ent sem.EntityID, t sem.TypeID, need map[sem.EntityID]map[sem.EntityID]bool) []sem.TypeID {
	r := s.p.R
	drop := r.TypeDecl(ent).Drop
	if drop == 0 || len(need[drop]) == 0 {
		return nil
	}
	bound, ok := r.ReceiverArgs(drop, t)
	if !ok {
		return []sem.TypeID{t}
	}
	var out []sem.TypeID
	for _, p := range r.Fn(drop).TypeParams {
		if need[drop][p] {
			out = append(out, bound[p])
		}
	}
	return out
}

func (s *specializer) shapeArgs(gen sem.EntityID, args []sem.TypeID) bool {
	_, ok := s.shapeArgsOnly(gen, args)
	return ok
}

func (s *specializer) shapeArgsOnly(gen sem.EntityID, args []sem.TypeID) ([]sem.TypeID, bool) {
	r := s.p.R
	info := r.Fn(gen)
	shapeParams := s.shapeParams()[gen]
	if len(shapeParams) == 0 {
		return nil, false
	}
	out := make([]sem.TypeID, len(args))
	selected := false
	for i, p := range info.TypeParams {
		if !shapeParams[p] || r.IsLifetimeParam(p) {
			continue
		}
		if i >= len(args) || !instance.Concrete(r, args[i]) {
			return nil, false
		}
		out[i] = args[i]
		selected = true
	}
	return out, selected
}

// instanceOnly lists the generic bodies a shape parameter makes run only as instances.
func (s *specializer) instanceOnly() map[sem.EntityID]bool {
	out := map[sem.EntityID]bool{}
	for ent, params := range s.shapeParams() {
		if len(params) > 0 {
			out[ent] = true
		}
	}
	return out
}
