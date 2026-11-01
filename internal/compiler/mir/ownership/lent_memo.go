package ownership

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// LentMemo caches LentRefs per function; it is never invalidated.
type LentMemo map[*midir.Func][]bool

func (m *LentMemo) Of(r *sem.Result, f *midir.Func) []bool {
	if out, ok := (*m)[f]; ok {
		return out
	}
	out := LentRefs(r, f)
	if *m == nil {
		*m = LentMemo{}
	}
	(*m)[f] = out
	return out
}
