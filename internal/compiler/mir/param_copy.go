package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// ParamCopyFact is a copy of type parameter K; Elem marks an array element handed out in an Option.
type ParamCopyFact struct {
	K    int
	Elem bool
}

func (p *Program) ParamCopy(f *midir.Func, in midir.Inst) (ParamCopyFact, bool) {
	if !p.DeclaredNoalloc(f) {
		return ParamCopyFact{}, false
	}
	if elem, ok := p.elemCopy(f, in); ok {
		k, own := p.OwnParamIndex(f, elem)
		return ParamCopyFact{K: k, Elem: true}, own
	}
	if in.Op != midir.OpShare {
		return ParamCopyFact{}, false
	}
	k, own := p.OwnParamIndex(f, f.Locals[in.Args[0]].Type)
	return ParamCopyFact{K: k}, own
}

func (p *Program) OwnParamIndex(f *midir.Func, t sem.TypeID) (int, bool) {
	if f.Ent == 0 || p.R.Types.Kind(t) != sem.KParam {
		return 0, false
	}
	k := slices.Index(p.R.Fn(f.Ent).TypeParams, p.R.Types.Node(t).Ent)
	return k, k >= 0
}
