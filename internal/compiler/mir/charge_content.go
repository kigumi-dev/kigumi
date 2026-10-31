package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// contentOps are the std ops that hand out a copy of a box's content.
var contentOps = map[string]bool{"BOX_GET": true}

// contentCopy is a call whose result is a copy of the content that allocates for the copy.
func (p *Program) contentCopy(f *midir.Func, in midir.Inst) (sem.TypeID, bool) {
	if in.Op != midir.OpCall || in.DPrim != opdef.DPNone || !f.DirectPlanned() || !ownquery.IsStdPrimitive(p.R, in.Ent) {
		return 0, false
	}
	if id, ok := in.StdOp(); !ok || !contentOps[opdef.StdOpNames[id]] {
		return 0, false
	}
	t := f.Locals[in.Dst].Type
	return t, p.R.CopyAllocates(t)
}
