package ownership

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// LentRefs computes which references hold no count in the current body.
func LentRefs(r *sem.Result, f *midir.Func) []bool {
	out := make([]bool, len(f.Locals))
	for _, prm := range f.Params {
		out[prm] = r.Types.Kind(f.Locals[prm].Type) == sem.KRef
	}
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if !in.Op.Yields() || out[in.Dst] || !lendsOn(in, out) {
					continue
				}
				out[in.Dst], changed = true, true
			}
		}
	}
	return out
}

func lendsOn(in midir.Inst, out []bool) bool {
	switch in.Op {
	case midir.OpBorrow, midir.OpLendBox:
		return true
	case midir.OpAlias, midir.OpMove:
		return len(in.Args) == 1 && out[in.Args[0]]
	case midir.OpCopy:
		return !in.Share && len(in.Args) == 1 && out[in.Args[0]]
	case midir.OpUnboxDirect:
		return in.Lent && out[in.Args[0]]
	}
	return false
}

// DeclaredParamEnt is declaredParam's parameter itself, for a diagnostic naming it.
func DeclaredParamEnt(r *sem.Result, in midir.Inst, i int) (sem.EntityID, bool) {
	switch in.Op {
	case midir.OpCall:
	case midir.OpCallValue:
		i--
	default:
		return 0, false
	}
	if in.Ent == 0 || r.Entity(in.Ent).Kind != sem.EntFn {
		return 0, false
	}
	info := r.Fn(in.Ent)
	if info.Recv != sem.RecvNone {
		i--
	}
	if i < 0 || i >= len(info.Params) {
		return 0, false
	}
	return info.Params[i], true
}
