package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// PlanDirect, not the backend, decides every crossing between the unboxed and boxed forms.
func PlanDirect(p *Program) error {
	p.Entries = nil
	p.lendByValue()
	fs := p.allFuncs()
	pins := make([][]bool, len(fs))
	for i, f := range fs {
		p.markDirectPrims(f)
		pins[i] = p.unpinEnv(f, p.pinned(f))
		f.DSig = p.directSig(f, pins[i])
	}
	for i, f := range fs {
		f.Direct = p.classifyDirect(f, pins[i])
		p.settlePrims(f, pins[i])
	}
	for _, f := range fs {
		settleElided(f)
		p.crossFunc(f)
		p.settleValueArgs(f)
		p.markFixed(f)
		if f.Async {
			p.slotHeldBoxes(f)
			p.layoutFrame(f)
		}
	}
	p.buildEntries(fs)
	p.chargeCopies()
	return nil
}

func (p *Program) crossFunc(f *midir.Func) {
	countless := p.countlessRefs(f)
	for bid := range f.Blocks {
		blk := &f.Blocks[bid]
		out := make([]midir.Inst, 0, len(blk.Insts))
		for _, in := range blk.Insts {
			out = p.crossInst(f, in, out, countless)
		}
		blk.Insts = out
		p.crossReturn(f, blk, countless)
	}
}

func (p *Program) crossInst(f *midir.Func, in midir.Inst, out []midir.Inst, countless []bool) []midir.Inst {
	src := in
	for i, a := range src.Args {
		cross, ct, ok := p.argCross(f, src, i, a)
		if !ok {
			continue
		}
		t := crossTemp(f, ct, cross == midir.OpUnboxDirect)
		out = append(out, midir.Inst{Op: cross, Dst: t, Type: ct, Args: []midir.LocalID{a}, Node: in.Node, Lent: countless[a]})
		in.Args = replaceArg(in.Args, i, t)
	}
	cross, ct, ok := p.dstCross(f, in)
	if !ok {
		return append(out, in)
	}
	dst := in.Dst
	in.Dst = crossTemp(f, ct, cross == midir.OpBoxDirect)
	out = append(out, in)
	return append(out, midir.Inst{Op: cross, Dst: dst, Type: ct, Args: []midir.LocalID{in.Dst}, Node: in.Node})
}

func (p *Program) argCross(f *midir.Func, in midir.Inst, i int, a midir.LocalID) (midir.Opcode, sem.TypeID, bool) {
	want := p.argPos(f, in, i)
	switch {
	case f.IsDirect(a) && want == 0:
		return midir.OpBoxDirect, f.Locals[a].Type, true
	case !f.IsDirect(a) && want != 0:
		return midir.OpUnboxDirect, want, true
	}
	return midir.OpNop, 0, false
}

func (p *Program) dstCross(f *midir.Func, in midir.Inst) (midir.Opcode, sem.TypeID, bool) {
	if !in.Op.Yields() {
		return midir.OpNop, 0, false
	}
	want := p.dstPos(f, in)
	switch {
	case f.IsDirect(in.Dst) && want == 0:
		return midir.OpUnboxDirect, f.Locals[in.Dst].Type, true
	case !f.IsDirect(in.Dst) && want != 0:
		return midir.OpBoxDirect, want, true
	}
	return midir.OpNop, 0, false
}

func (p *Program) crossReturn(f *midir.Func, blk *midir.Block, countless []bool) {
	if blk.Term.Op != midir.TermReturn {
		return
	}
	a := blk.Term.Args[0]
	cross, ct, ok := p.returnCross(f, a)
	if !ok {
		return
	}
	t := crossTemp(f, ct, cross == midir.OpUnboxDirect)
	blk.Insts = append(blk.Insts, midir.Inst{Op: cross, Dst: t, Type: ct, Args: []midir.LocalID{a}, Lent: countless[a]})
	blk.Term.Args = replaceArg(blk.Term.Args, 0, t)
}

// The result is one more position the signature fixes, so it crosses the same way an argument does.
func (p *Program) returnCross(f *midir.Func, a midir.LocalID) (midir.Opcode, sem.TypeID, bool) {
	carried := f.DSig.Result()
	switch {
	case f.IsDirect(a) && carried == 0:
		return midir.OpBoxDirect, f.Locals[a].Type, true
	case !f.IsDirect(a) && carried != 0:
		return midir.OpUnboxDirect, carried, true
	}
	return midir.OpNop, 0, false
}
