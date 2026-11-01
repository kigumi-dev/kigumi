package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *Program) DeclaredNoalloc(f *midir.Func) bool {
	if f.Ent == 0 || p.R.Entity(f.Ent).Kind != sem.EntFn {
		return false
	}
	return p.R.Fn(f.Ent).Declared&sem.EffNoalloc != 0
}
