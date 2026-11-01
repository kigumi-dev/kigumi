package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// A capture arrives unboxed; a parameter and the result do too unless the body lends the parameter.
func (p *planner) envSig(f *midir.Func, pin []bool) *midir.DirectSig {
	s := &midir.DirectSig{Params: make([]sem.TypeID, len(f.Params))}
	caps := len(p.EnvTypes(f))
	for i, prm := range f.Params {
		if t := f.Locals[prm].Type; (i < caps || !pin[prm]) && p.R.DirectAgg(t) {
			s.Params[i] = t
		}
	}
	if p.R.DirectAgg(f.Ret) {
		s.Ret = f.Ret
	}
	return s
}

func (p *planner) markValueCall(f *midir.Func, in midir.Inst, out []bool) {
	t := f.Locals[in.Args[0]].Type
	if !p.R.DirectClosure(t) {
		return
	}
	body, ok := p.ByEnt[p.R.Types.Node(t).Ent]
	if !ok {
		return
	}
	caps := len(p.EnvTypes(body))
	for i, a := range in.Args[1:] {
		if body.DSig.Param(caps+i) != 0 {
			out[a] = true
		}
	}
	if body.DSig.Result() != 0 {
		out[in.Dst] = true
	}
}

// The environment hands the capture over unboxed and the body boxes it on entry.
func (p *planner) unpinEnv(f *midir.Func, pin []bool) []bool {
	if !p.EnvABI(f) {
		return pin
	}
	var pre []midir.Inst
	for i, t := range p.EnvTypes(f) {
		prm := f.Params[i]
		if !pin[prm] || !p.R.DirectAgg(t) {
			continue
		}
		f.Locals = append(f.Locals, midir.Local{Name: f.Locals[prm].Name + "$env", Type: t})
		if len(f.Slot) > 0 {
			f.Slot = append(f.Slot, -1)
		}
		in := midir.LocalID(len(f.Locals) - 1)
		pin = append(pin, false)
		f.Params[i] = in
		pre = append(pre, midir.Inst{Op: midir.OpBoxDirect, Dst: prm, Type: t, Args: []midir.LocalID{in}})
	}
	if len(pre) > 0 {
		f.Blocks[0].Insts = append(pre, f.Blocks[0].Insts...)
	}
	return pin
}
