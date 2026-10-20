package mir

import (
	"strconv"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// placeName spells a field read as the path it projects; a pattern's own text would name the binding instead.
func (pl *places) placeName(p *Program, f *Func, t *syntax.Tree, in Inst) string {
	if in.Op == OpPayload {
		if n, ok := pl.name[pl.canon(in.Args[0])]; ok {
			return n
		}
		return f.Locals[pl.canon(in.Args[0])].Name
	}
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

func (pl *places) projects(t *syntax.Tree, in Inst) bool {
	switch in.Op {
	case OpField, OpFieldMove:
		return true
	case OpPayload:
		return in.Node != 0 && t.Kind(in.Node).IsPattern()
	}
	return false
}

func (pl *places) step(in Inst) string {
	if in.Op == OpPayload {
		return "p" + strconv.Itoa(in.Index)
	}
	return strconv.Itoa(in.Index)
}
