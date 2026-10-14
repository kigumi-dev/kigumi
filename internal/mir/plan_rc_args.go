package mir

import "slices"

func (p *Program) shareArgs(f *Func, in *Inst, st *rcState, out *[]Inst, w *shareFacts, at Point) []LocalID {
	if len(in.Args) == 0 {
		return nil
	}
	var back []LocalID
	args := append([]LocalID(nil), in.Args...)
	for i, a := range args {
		kind := p.sinkAt(f, *in, i, w.sum)
		switch {
		case kind == keepsArg && w.sinkShared(*in, i, at):
			back = append(back, p.copyArg(f, in, a, &args[i], out))
		case p.inPlace(*in, i) || st.borrowed[a]:
		case p.mustCopy(f, *in, st.origin(a)):
			args[i] = p.holdArg(f, *in, i, kind, st, out, w, at)
			if !p.consumes(f, *in, i) {
				back = append(back, args[i])
			}
		case kind == takesArg && w.sinkShared(*in, i, at), w.heldAcrossWrite(*in, i):
			t := p.copyArg(f, in, a, &args[i], out)
			switch {
			case !p.consumes(f, *in, i):
				back = append(back, t)
			case !w.liveness().after(a, at.Block, at.Inst) && !slices.Contains(back, a):
				back = append(back, a)
			}
		}
	}
	in.Args = args
	return back
}

func (p *Program) copyArg(f *Func, in *Inst, a LocalID, slot *LocalID, out *[]Inst) LocalID {
	t := f.newTemp(f.Locals[a].Type)
	*out = append(*out, Inst{Op: OpShare, Dst: t, Args: []LocalID{a}, Copy: true, IfWritable: p.R.WritableByParam(f.Locals[a].Type), Own: true, Node: in.Node})
	*slot = t
	return t
}

func (p *Program) holdArg(f *Func, in Inst, i int, kind sinkKind, st *rcState, out *[]Inst, w *shareFacts, at Point) LocalID {
	a := in.Args[i]
	o := st.origin(a)
	t := f.newTemp(f.Locals[a].Type)
	last := p.handsOver(f, in, i, kind, st, w, at)
	src := a
	if last {
		src = o
	}
	retain := !last && kind == noSink && p.retainsArg(f, in, i, o, w)
	*out = append(*out, Inst{Op: OpShare, Dst: t, Args: []LocalID{src}, Copy: p.copyable(f.Locals[o].Type), Node: in.Node, Last: last, Retain: retain})
	return t
}

func (p *Program) handsOver(f *Func, in Inst, i int, kind sinkKind, st *rcState, w *shareFacts, at Point) bool {
	a := in.Args[i]
	o := st.origin(a)
	if !p.movable(f, o) || w.observed(o, at) || w.liveness().after(a, at.Block, at.Inst) {
		return false
	}
	n := 0
	for _, b := range in.Args {
		if st.origin(b) == o {
			n++
		}
	}
	if n > 1 {
		return false
	}
	if kind != noSink && w.sinkShared(in, i, at) || w.heldAcrossWrite(in, i) {
		return false
	}
	return (w.ownsBox(o) || w.leftBehind(in, i, at)) && w.freshAlias(a, o, at)
}

func (w *shareFacts) freshAlias(a, o LocalID, at Point) bool {
	blk := w.blocks[at.Block]
	v := a
	for k := at.Inst - 1; k >= 0 && v != o; k-- {
		x := blk.Insts[k]
		switch {
		case !x.Op.Yields():
		case x.Dst == o:
			return false
		case x.Dst == v:
			if x.Op != OpAlias && (x.Op != OpCopy || x.Share) || len(x.Args) != 1 {
				return false
			}
			v = x.Args[0]
		}
	}
	return v == o
}
