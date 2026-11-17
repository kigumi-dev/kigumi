package vm

import "kigumi/internal/compiler/sem"

func sameSessionType(old, next *sem.Result, a, b sem.TypeID) bool {
	work := [][2]sem.TypeID{{a, b}}
	seen := map[[2]sem.TypeID]bool{}
	for len(work) > 0 {
		pair := work[len(work)-1]
		work = work[:len(work)-1]
		if seen[pair] {
			continue
		}
		seen[pair] = true
		a, b := pair[0], pair[1]
		if a == 0 || b == 0 {
			if a != b {
				return false
			}
			continue
		}
		x, y := old.Types.Node(a), next.Types.Node(b)
		if x.Kind != y.Kind || x.Flags != y.Flags || len(x.Args) != len(y.Args) || entityKey(old, x.Ent) != entityKey(next, y.Ent) {
			return false
		}
		switch x.Kind {
		case sem.KPrim, sem.KUntyped:
			if a != b {
				return false
			}
		case sem.KConst:
			v, _ := old.Types.ConstValueBig(a)
			w, _ := next.Types.ConstValueBig(b)
			if v.Cmp(w) != 0 {
				return false
			}
		case sem.KVar:
			if x.Var != y.Var {
				return false
			}
		}
		work = append(work, [2]sem.TypeID{x.Elem, y.Elem}, [2]sem.TypeID{x.Lifetime, y.Lifetime})
		for i := range x.Args {
			if x.Kind == sem.KFn && old.Types.FnParamEscapes(a, i) != next.Types.FnParamEscapes(b, i) {
				return false
			}
			work = append(work, [2]sem.TypeID{x.Args[i], y.Args[i]})
		}
	}
	return true
}
