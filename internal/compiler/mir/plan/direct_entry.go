package plan

import (
	"strconv"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// buildEntries gives each direct-signature function the all-boxed entry its boxed callers run.
func (p *planner) buildEntries(fs []*midir.Func) {
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

func (p *planner) entryBody(f *midir.Func) *midir.Func {
	b := &midir.Func{Name: f.Name + "$boxed", File: f.File, Ret: f.Ret, Wraps: f}
	caps := 0
	if p.EnvABI(f) {
		caps = len(p.EnvTypes(f))
	}
	// The entry owns every capture the adapter hands it; a lent parameter stays lent.
	for i, prm := range f.Params {
		b.Params = append(b.Params, entryLocal(b, f.Locals[prm].Type, false))
		b.Locals[len(b.Locals)-1].Borrowed = i >= caps && f.Locals[prm].Borrowed
	}
	var insts []midir.Inst
	r := entryLocal(b, f.Ret, false)
	if p.EnvABI(f) {
		c := entryLocal(b, p.R.Types.Closure(f.Ent), true)
		// A scalar capture is read into a copy of its own, the way plan-rc feeds a closure literal.
		args, held := make([]midir.LocalID, caps), append([]midir.LocalID{}, b.Params[:caps]...)
		for i, prm := range b.Params[:caps] {
			args[i] = prm
			if t := b.Locals[prm].Type; !p.R.DirectAgg(t) {
				args[i] = entryLocal(b, t, false)
				held = append(held, args[i])
				insts = append(insts, midir.Inst{Op: midir.OpShare, Copy: true, Dst: args[i], Args: []midir.LocalID{prm}})
			}
		}
		insts = append(insts, midir.Inst{Op: midir.OpClosure, Dst: c, Ent: f.Ent, Args: args})
		for _, l := range held {
			insts = append(insts, midir.Inst{Op: midir.OpRelease, Args: []midir.LocalID{l}})
		}
		insts = append(insts,
			midir.Inst{Op: midir.OpCallValue, Dst: r, Args: append([]midir.LocalID{c}, b.Params[caps:]...)},
			midir.Inst{Op: midir.OpRelease, Args: []midir.LocalID{c}})
	} else {
		insts = append(insts, midir.Inst{Op: midir.OpCall, Dst: r, Ent: f.Ent, Args: append([]midir.LocalID{}, b.Params...)})
	}
	b.Blocks = []midir.Block{{Insts: insts, Term: midir.Term{Op: midir.TermReturn, Args: []midir.LocalID{r}}}}
	return b
}

func entryLocal(b *midir.Func, t sem.TypeID, direct bool) midir.LocalID {
	b.Locals = append(b.Locals, midir.Local{Name: "e" + strconv.Itoa(len(b.Locals)), Type: t})
	b.Direct = append(b.Direct, direct)
	return midir.LocalID(len(b.Locals) - 1)
}
