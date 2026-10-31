package sem

import "kigumi/internal/compiler/syntax"

func (r *Result) checkStrayBang(fn EntityID, mods uint32) {
	if mods&syntax.ModStrayBang == 0 {
		return
	}
	e := r.Entities[fn]
	r.errAt(e.File, e.Node, cStrayBangReturn, r.TypeString(r.Types.Node(r.Fn(fn).Sig).Elem))
}
