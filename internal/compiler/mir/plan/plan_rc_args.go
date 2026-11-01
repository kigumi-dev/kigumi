package plan

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *planner) shareArgs(f *midir.Func, in *midir.Inst, st *rcState, out *[]midir.Inst, w *shareFacts, at Point) []midir.LocalID {
	if len(in.Args) == 0 {
		return nil
	}
	var back []midir.LocalID
	args := append([]midir.LocalID(nil), in.Args...)
	for i, a := range args {
		kind := p.sinkAt(f, *in, i, w.sum)
		switch {
		case kind == keepsArg && w.sinkShared(*in, i, at):
			back = append(back, p.copyArg(f, in, a, &args[i], out))
		case p.InPlace(*in, i) || st.borrowed[a]:
		case p.mustCopy(f, *in, st.origin(a)):
			args[i] = p.holdArg(f, *in, i, kind, st, out, w, at)
			if !p.Consumes(f, *in, i) {
				back = append(back, args[i])
			}
		case kind == takesArg && w.sinkShared(*in, i, at), w.heldAcrossWrite(*in, i), w.storesPlaceCopy(*in, i):
			t := p.copyArg(f, in, a, &args[i], out)
			switch {
			case !p.Consumes(f, *in, i):
				back = append(back, t)
			case !w.liveness().after(a, at.Block, at.Inst) && !slices.Contains(back, a):
				back = append(back, a)
			}
		}
	}
	in.Args = args
	return back
}

func (p *planner) copyArg(f *midir.Func, in *midir.Inst, a midir.LocalID, slot *midir.LocalID, out *[]midir.Inst) midir.LocalID {
	t := newTemp(f, f.Locals[a].Type)
	*out = append(*out, midir.Inst{Op: midir.OpShare, Dst: t, Args: []midir.LocalID{a}, Copy: true, IfWritable: p.R.WritableByParam(f.Locals[a].Type), Own: true, Node: in.Node})
	*slot = t
	return t
}

func (p *planner) holdArg(f *midir.Func, in midir.Inst, i int, kind sinkKind, st *rcState, out *[]midir.Inst, w *shareFacts, at Point) midir.LocalID {
	a := in.Args[i]
	o := st.origin(a)
	t := newTemp(f, f.Locals[a].Type)
	last := p.handsOver(f, in, i, kind, st, w, at)
	src := a
	if last {
		src = o
	}
	retain := !last && kind == noSink && p.retainsArg(f, in, i, o, w)
	*out = append(*out, midir.Inst{Op: midir.OpShare, Dst: t, Args: []midir.LocalID{src}, Copy: p.copyable(f.Locals[o].Type), Node: in.Node, Last: last, Retain: retain})
	return t
}

func (p *planner) handsOver(f *midir.Func, in midir.Inst, i int, kind sinkKind, st *rcState, w *shareFacts, at Point) bool {
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

func (w *shareFacts) freshAlias(a, o midir.LocalID, at Point) bool {
	blk := w.blocks[at.Block]
	v := a
	for k := at.Inst - 1; k >= 0 && v != o; k-- {
		x := blk.Insts[k]
		switch {
		case !x.Op.Yields():
		case x.Dst == o:
			return false
		case x.Dst == v:
			if x.Op != midir.OpAlias && (x.Op != midir.OpCopy || x.Share) || len(x.Args) != 1 {
				return false
			}
			v = x.Args[0]
		}
	}
	return v == o
}

func returned(f *midir.Func, l midir.LocalID) bool {
	for _, b := range f.Blocks {
		if b.Term.Op == midir.TermReturn && len(b.Term.Args) > 0 && b.Term.Args[0] == l {
			return true
		}
	}
	return false
}

// countless reports whether l holds no count: a borrow, an alias of one, or a lent reference parameter.
func (p *planner) countless(f *midir.Func, st *rcState, l midir.LocalID) bool {
	if st.borrowed[l] || st.viaBorrow[l] {
		return true
	}
	return !f.Locals[l].Cell && p.R.Types.Kind(f.Locals[l].Type) == sem.KRef && slices.Contains(f.Params, l)
}
