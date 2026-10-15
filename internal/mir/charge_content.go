package mir

import "kigumi/internal/sem"

// contentOps are the std ops that hand out a copy of a box's content.
var contentOps = map[string]bool{"BOX_GET": true}

// contentCopy is a call whose result is a copy of the content that allocates for the copy.
func (p *Program) contentCopy(f *Func, in Inst) (sem.TypeID, bool) {
	if in.Op != OpCall || in.DPrim != DPNone || !f.DirectPlanned() || !p.IsStdPrimitive(in.Ent) {
		return 0, false
	}
	if id, ok := in.StdOp(); !ok || !contentOps[StdOpNames[id]] {
		return 0, false
	}
	t := f.Locals[in.Dst].Type
	return t, p.R.CopyAllocates(t)
}
