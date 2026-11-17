package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (s *specializer) simdAsync(gen sem.EntityID) bool {
	r := s.p.R
	if gen == 0 || r.Entity(gen).Kind != sem.EntFn {
		return false
	}
	f := r.Fn(gen)
	return f.Body != 0 && len(f.TypeParams) > 0 && f.Declared&sem.EffAsync != 0 && f.Abi == "" && f.Export == ""
}

func (s *specializer) callBinding(f *midir.Func, in midir.Inst) midir.Inst {
	if in.Op != midir.OpFnItem || !in.Async {
		return in
	}
	if s.futures == nil {
		s.futures = map[*midir.Func]map[midir.LocalID]midir.Inst{}
	}
	bindings, ok := s.futures[f]
	if !ok {
		bindings = map[midir.LocalID]midir.Inst{}
		defs := singleDefs(f)
		for _, block := range f.Blocks {
			for _, call := range block.Insts {
				if call.Op != midir.OpFutureNew || len(call.Args) == 0 {
					continue
				}
				if item := defs[call.Args[0]]; item != nil && item.Op == midir.OpFnItem && item.Async {
					call.Ent, call.Args = item.Ent, call.Args[1:]
					bindings[item.Dst] = call
				}
			}
		}
		s.futures[f] = bindings
	}
	if call, ok := bindings[in.Dst]; ok {
		return call
	}
	return in
}
