package mir

import "kigumi/internal/compiler/sem"

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
		f.settleElided()
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

func (p *Program) crossFunc(f *Func) {
	countless := p.countlessRefs(f)
	for bid := range f.Blocks {
		blk := &f.Blocks[bid]
		out := make([]Inst, 0, len(blk.Insts))
		for _, in := range blk.Insts {
			out = p.crossInst(f, in, out, countless)
		}
		blk.Insts = out
		p.crossReturn(f, blk, countless)
	}
}

func (p *Program) crossInst(f *Func, in Inst, out []Inst, countless []bool) []Inst {
	src := in
	for i, a := range src.Args {
		cross, ct, ok := p.argCross(f, src, i, a)
		if !ok {
			continue
		}
		t := f.temp(ct, cross == OpUnboxDirect)
		out = append(out, Inst{Op: cross, Dst: t, Type: ct, Args: []LocalID{a}, Node: in.Node, Lent: countless[a]})
		in.Args = replaceArg(in.Args, i, t)
	}
	cross, ct, ok := p.dstCross(f, in)
	if !ok {
		return append(out, in)
	}
	dst := in.Dst
	in.Dst = f.temp(ct, cross == OpBoxDirect)
	out = append(out, in)
	return append(out, Inst{Op: cross, Dst: dst, Type: ct, Args: []LocalID{in.Dst}, Node: in.Node})
}

func (p *Program) argCross(f *Func, in Inst, i int, a LocalID) (Opcode, sem.TypeID, bool) {
	want := p.argPos(f, in, i)
	switch {
	case f.IsDirect(a) && want == 0:
		return OpBoxDirect, f.Locals[a].Type, true
	case !f.IsDirect(a) && want != 0:
		return OpUnboxDirect, want, true
	}
	return OpNop, 0, false
}

func (p *Program) dstCross(f *Func, in Inst) (Opcode, sem.TypeID, bool) {
	if !in.Op.Yields() {
		return OpNop, 0, false
	}
	want := p.dstPos(f, in)
	switch {
	case f.IsDirect(in.Dst) && want == 0:
		return OpUnboxDirect, f.Locals[in.Dst].Type, true
	case !f.IsDirect(in.Dst) && want != 0:
		return OpBoxDirect, want, true
	}
	return OpNop, 0, false
}

func (p *Program) crossReturn(f *Func, blk *Block, countless []bool) {
	if blk.Term.Op != TermReturn {
		return
	}
	a := blk.Term.Args[0]
	cross, ct, ok := p.returnCross(f, a)
	if !ok {
		return
	}
	t := f.temp(ct, cross == OpUnboxDirect)
	blk.Insts = append(blk.Insts, Inst{Op: cross, Dst: t, Type: ct, Args: []LocalID{a}, Lent: countless[a]})
	blk.Term.Args = replaceArg(blk.Term.Args, 0, t)
}

// The result is one more position the signature fixes, so it crosses the same way an argument does.
func (p *Program) returnCross(f *Func, a LocalID) (Opcode, sem.TypeID, bool) {
	carried := f.DSig.result()
	switch {
	case f.IsDirect(a) && carried == 0:
		return OpBoxDirect, f.Locals[a].Type, true
	case !f.IsDirect(a) && carried != 0:
		return OpUnboxDirect, carried, true
	}
	return OpNop, 0, false
}
