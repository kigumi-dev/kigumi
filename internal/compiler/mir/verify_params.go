package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *verifier) verifyParams(f *midir.Func) *Error {
	if f.Ent == 0 || p.R.Entity(f.Ent).Kind != sem.EntFn {
		return nil
	}
	info := p.R.Fn(f.Ent)
	var want []sem.EntityID
	if info.Recv != sem.RecvNone {
		want = append(want, info.SelfParam)
	}
	want = append(want, info.Params...)
	for i, ent := range want {
		if i >= len(f.Params) {
			return &Error{Rule: "calls", Msg: "takes " + itoa(len(f.Params)) + " parameters, but " + p.R.Entity(f.Ent).Name + " declares " + itoa(len(want))}
		}
		if !declaresParam(f, f.Params[i], ent) {
			return &Error{Rule: "calls", Msg: "parameter " + itoa(i) + " is %" + f.Locals[f.Params[i]].Name + ", not the " + p.R.Entity(ent).Name + " that " + p.R.Entity(f.Ent).Name + " declares there"}
		}
	}
	return nil
}

func declaresParam(f *midir.Func, prm midir.LocalID, ent sem.EntityID) bool {
	if f.Locals[prm].Ent == ent {
		return true
	}
	if len(f.Blocks) == 0 {
		return false
	}
	for _, in := range f.Blocks[0].Insts {
		if in.Op == midir.OpNewCell && len(in.Args) == 1 && in.Args[0] == prm && f.Locals[in.Dst].Ent == ent {
			return true
		}
	}
	return false
}
