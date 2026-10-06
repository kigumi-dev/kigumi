package mir

import (
	"fmt"

	"kigumi/internal/sem"
)

// PlanDirect, not the backend, decides every crossing between the unboxed and boxed forms.
func PlanDirect(p *Program) error {
	fs := p.allFuncs()
	for _, f := range fs {
		f.DSig = p.directSig(f)
	}
	for _, f := range fs {
		f.Direct = p.classifyDirect(f)
	}
	for _, f := range fs {
		p.crossFunc(f)
	}
	return nil
}

func (f *Func) directTemp(t sem.TypeID) LocalID {
	f.Locals = append(f.Locals, Local{Name: fmt.Sprintf("x%d", len(f.Locals)), Type: t})
	f.Direct = append(f.Direct, false)
	return LocalID(len(f.Locals) - 1)
}

func (p *Program) crossFunc(f *Func) {
	if !anyDirect(f) {
		return
	}
	for bid := range f.Blocks {
		blk := &f.Blocks[bid]
		out := make([]Inst, 0, len(blk.Insts))
		for _, in := range blk.Insts {
			out = p.crossInst(f, in, out)
		}
		blk.Insts = out
		p.crossReturn(f, blk)
	}
}

func anyDirect(f *Func) bool {
	for _, d := range f.Direct {
		if d {
			return true
		}
	}
	return false
}

func (p *Program) crossInst(f *Func, in Inst, out []Inst) []Inst {
	src := in
	for i, a := range src.Args {
		if !f.IsDirect(a) || p.directArg(f, src, i) {
			continue
		}
		t := f.directTemp(f.Locals[a].Type)
		out = append(out, Inst{Op: OpBoxDirect, Dst: t, Type: f.Locals[a].Type, Args: []LocalID{a}, Node: in.Node})
		in.Args = replaceArg(in.Args, i, t)
	}
	if !in.Op.Yields() || !f.IsDirect(in.Dst) || p.directDst(f, in) {
		return append(out, in)
	}
	dst := in.Dst
	in.Dst = f.directTemp(f.Locals[dst].Type)
	out = append(out, in)
	return append(out, Inst{Op: OpUnboxDirect, Dst: dst, Type: f.Locals[dst].Type, Args: []LocalID{in.Dst}, Node: in.Node})
}

// replaceArg copies on first write: the builder shares argument slices.
func replaceArg(args []LocalID, i int, l LocalID) []LocalID {
	cp := make([]LocalID, len(args))
	copy(cp, args)
	cp[i] = l
	return cp
}

func (p *Program) crossReturn(f *Func, blk *Block) {
	if blk.Term.Op != TermReturn || f.DSig.result() != 0 {
		return
	}
	a := blk.Term.Args[0]
	if !f.IsDirect(a) {
		return
	}
	t := f.directTemp(f.Locals[a].Type)
	blk.Insts = append(blk.Insts, Inst{Op: OpBoxDirect, Dst: t, Type: f.Locals[a].Type, Args: []LocalID{a}})
	blk.Term.Args = replaceArg(blk.Term.Args, 0, t)
}

func (p *Program) directForm(f *Func, in Inst) bool {
	switch in.Op {
	case OpRecord, OpAlias, OpMove, OpCopy, OpShare:
		return f.IsDirect(in.Dst)
	case OpField, OpFieldMove, OpSetField, OpDrop, OpRelease:
		return f.IsDirect(in.Args[0])
	case OpBoxDirect, OpUnboxDirect:
		return true
	}
	return false
}

// A direct signature covers each parameter independently, so a call checks position by position.
func (p *Program) directArg(f *Func, in Inst, i int) bool {
	if in.Op == OpCall {
		fn, ok := p.ByEnt[in.Ent]
		return ok && fn.DSig.Param(i) != 0
	}
	return p.directForm(f, in)
}

func (p *Program) directDst(f *Func, in Inst) bool {
	if in.Op == OpCall {
		fn, ok := p.ByEnt[in.Ent]
		return ok && fn.DSig.result() != 0
	}
	return p.directForm(f, in)
}
