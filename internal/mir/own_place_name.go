package mir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// placeName spells a field read as the path it projects; a pattern's own text would name the binding instead.
func (pl *places) placeName(p *Program, f *Func, t *syntax.Tree, in Inst) string {
	sp := t.Span(in.Node)
	text := string(t.File.Src[sp.Start:sp.End])
	if !t.Kind(in.Node).IsPattern() {
		return text
	}
	ent := p.R.Types.Node(f.Locals[in.Args[0]].Type).Ent
	if ent == 0 || p.R.Entity(ent).Kind != sem.EntType {
		return text
	}
	fields := p.R.TypeDecl(ent).Fields
	if in.Index >= len(fields) {
		return text
	}
	base := f.Locals[in.Args[0]].Name
	if n, ok := pl.name[pl.canon(in.Args[0])]; ok {
		base = n
	}
	return base + "." + p.R.Entity(fields[in.Index]).Name
}
