package mir

import "kigumi/internal/sem"

// buildEntries gives each direct-signature function the all-boxed entry its boxed callers run.
func (p *Program) buildEntries(fs []*Func) {
	for _, f := range fs {
		if f.DSig == nil || f.Ent == 0 {
			continue
		}
		b := p.entryBody(f)
		f.Entry = b
		p.Entries = append(p.Entries, b)
		p.crossFunc(b)
	}
}

func (p *Program) entryBody(f *Func) *Func {
	b := &Func{Name: f.Name + "$boxed", File: f.File, Ret: f.Ret, Wraps: f}
	caps := 0
	if p.EnvABI(f) {
		caps = len(p.envTypes(f))
	}
	// The entry owns every capture the adapter hands it; a lent parameter stays lent.
	for i, prm := range f.Params {
		b.Params = append(b.Params, b.entryLocal(f.Locals[prm].Type, false))
		b.Locals[len(b.Locals)-1].Borrowed = i >= caps && f.Locals[prm].Borrowed
	}
	var insts []Inst
	r := b.entryLocal(f.Ret, false)
	if p.EnvABI(f) {
		c := b.entryLocal(p.R.Types.Closure(f.Ent), true)
		// A scalar capture is read into a copy of its own, the way plan-rc feeds a closure literal.
		args, held := make([]LocalID, caps), append([]LocalID{}, b.Params[:caps]...)
		for i, prm := range b.Params[:caps] {
			args[i] = prm
			if t := b.Locals[prm].Type; !p.R.DirectAgg(t) {
				args[i] = b.entryLocal(t, false)
				held = append(held, args[i])
				insts = append(insts, Inst{Op: OpShare, Copy: true, Dst: args[i], Args: []LocalID{prm}})
			}
		}
		insts = append(insts, Inst{Op: OpClosure, Dst: c, Ent: f.Ent, Args: args})
		for _, l := range held {
			insts = append(insts, Inst{Op: OpRelease, Args: []LocalID{l}})
		}
		insts = append(insts,
			Inst{Op: OpCallValue, Dst: r, Args: append([]LocalID{c}, b.Params[caps:]...)},
			Inst{Op: OpRelease, Args: []LocalID{c}})
	} else {
		insts = append(insts, Inst{Op: OpCall, Dst: r, Ent: f.Ent, Args: append([]LocalID{}, b.Params...)})
	}
	b.Blocks = []Block{{Insts: insts, Term: Term{Op: TermReturn, Args: []LocalID{r}}}}
	return b
}

func (b *Func) entryLocal(t sem.TypeID, direct bool) LocalID {
	b.Locals = append(b.Locals, Local{Name: "e" + itoa(len(b.Locals)), Type: t})
	b.Direct = append(b.Direct, direct)
	return LocalID(len(b.Locals) - 1)
}
