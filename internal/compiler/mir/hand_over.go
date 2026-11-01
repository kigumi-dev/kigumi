package mir

import midir "kigumi/internal/compiler/mir/ir"

// CrossHandsOver reports whether consuming the crossing gives its source's count back:
// a named local keeps its value, and a Lent crossing reads a reference that holds no count.
func (p *Program) CrossHandsOver(f *midir.Func, cross *midir.Inst) bool {
	return f.Locals[cross.Args[0]].Ent == 0 && !cross.Lent
}
