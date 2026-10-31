package mir

import "kigumi/internal/compiler/sem"

func (p *Program) EnvABI(f *Func) bool {
	if f.Ent == 0 || p.R.Entity(f.Ent).Kind != sem.EntClosure {
		return false
	}
	return p.R.DirectClosure(p.R.Types.Closure(f.Ent))
}

func (p *Program) envTypes(f *Func) []sem.TypeID {
	return p.R.AggFields(p.R.Types.Closure(f.Ent))
}

// A capture arrives unboxed; a parameter and the result do too unless the body lends the parameter.
func (p *Program) envSig(f *Func, pin []bool) *DirectSig {
	s := &DirectSig{Params: make([]sem.TypeID, len(f.Params))}
	caps := len(p.envTypes(f))
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

// A boxed value is called through its adapter, so only an unboxed one names its body.
func (p *Program) valueCallee(f *Func, in Inst) *Func {
	t := p.wholePos(f, in.Args[0])
	if t == 0 || !p.R.DirectClosure(t) {
		return nil
	}
	return p.ByEnt[p.R.Types.Node(t).Ent]
}

func (p *Program) valueArgPos(f *Func, in Inst, i int) sem.TypeID {
	body := p.valueCallee(f, in)
	if body == nil {
		return 0
	}
	return body.DSig.Param(len(p.envTypes(body)) + i - 1)
}

func (p *Program) markValueCall(f *Func, in Inst, out []bool) {
	t := f.Locals[in.Args[0]].Type
	if !p.R.DirectClosure(t) {
		return
	}
	body, ok := p.ByEnt[p.R.Types.Node(t).Ent]
	if !ok {
		return
	}
	caps := len(p.envTypes(body))
	for i, a := range in.Args[1:] {
		if body.DSig.Param(caps+i) != 0 {
			out[a] = true
		}
	}
	if body.DSig.result() != 0 {
		out[in.Dst] = true
	}
}

// The environment hands the capture over unboxed and the body boxes it on entry.
func (p *Program) unpinEnv(f *Func, pin []bool) []bool {
	if !p.EnvABI(f) {
		return pin
	}
	var pre []Inst
	for i, t := range p.envTypes(f) {
		prm := f.Params[i]
		if !pin[prm] || !p.R.DirectAgg(t) {
			continue
		}
		f.Locals = append(f.Locals, Local{Name: f.Locals[prm].Name + "$env", Type: t})
		if len(f.Slot) > 0 {
			f.Slot = append(f.Slot, -1)
		}
		in := LocalID(len(f.Locals) - 1)
		pin = append(pin, false)
		f.Params[i] = in
		pre = append(pre, Inst{Op: OpBoxDirect, Dst: prm, Type: t, Args: []LocalID{in}})
	}
	if len(pre) > 0 {
		f.Blocks[0].Insts = append(pre, f.Blocks[0].Insts...)
	}
	return pin
}
