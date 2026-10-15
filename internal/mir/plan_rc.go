package mir

import (
	"cmp"
	"fmt"
	"slices"

	"kigumi/internal/sem"
)

// After PlanRC a backend translates opcodes one by one without consulting Plan.
func PlanRC(p *Program) error {
	fs := p.allFuncs()
	s := p.planShares(fs)
	for _, f := range fs {
		p.planRCFunc(f, s.facts[f])
	}
	return planAsync(p)
}

// rcState is the working memory PlanRC threads across one function's
// blocks, in the same block-array order the two backends walk today: which
// temporary stands for which named local (alias), and which temporaries a
// borrow produced (never copied, never re-aliased further).
type rcState struct {
	alias    map[LocalID]LocalID
	borrowed map[LocalID]bool
}

func (s *rcState) origin(l LocalID) LocalID {
	for {
		src, ok := s.alias[l]
		if !ok {
			return l
		}
		l = src
	}
}

func (p *Program) planRCFunc(f *Func, w *shareFacts) {
	f.rcBase = len(f.Locals)
	plan := p.planFunc(f)
	st := &rcState{alias: map[LocalID]LocalID{}, borrowed: map[LocalID]bool{}}
	for bid := range f.Blocks {
		p.planRCBlock(f, bid, plan, st, w)
	}
	splitEdges(f, plan)
}

func splitEdges(f *Func, plan *Plan) {
	edges := make([]Edge, 0, len(plan.Splits))
	for e := range plan.Splits {
		edges = append(edges, e)
	}
	slices.SortFunc(edges, func(a, b Edge) int { return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.Slot, b.Slot)) })
	for _, e := range edges {
		var insts []Inst
		for _, t := range plan.Splits[e] {
			insts = append(insts, Inst{Op: OpRelease, Args: []LocalID{t}})
		}
		to := f.Blocks[e.From].Term.Targets[e.Slot]
		f.Blocks = append(f.Blocks, Block{Insts: insts, Term: Term{Op: TermJump, Targets: []BlockID{to}}})
		f.Blocks[e.From].Term.Targets[e.Slot] = BlockID(len(f.Blocks) - 1)
	}
}

func (p *Program) planRCBlock(f *Func, bid int, plan *Plan, st *rcState, w *shareFacts) {
	blk := &f.Blocks[bid]
	orig := blk.Insts
	out := make([]Inst, 0, len(orig))
	for _, t := range plan.Releases[Point{bid, -1}] {
		out = append(out, Inst{Op: OpRelease, Args: []LocalID{t}})
	}
	for k := range orig {
		in, at := orig[k], Point{bid, k}
		release := p.shareArgs(f, &in, st, &out, w, at)
		release = append(release, p.copyBinding(f, &in, st, w, at)...)
		p.rewriteCopy(f, &in, plan, st, w, at)
		out = append(out, in)
		for _, t := range append(release, plan.Releases[at]...) {
			out = append(out, Inst{Op: OpRelease, Args: []LocalID{t}})
		}
	}
	release := p.shareReturn(f, &blk.Term, st, &out, w, Point{bid, len(orig)})
	for _, t := range append(release, plan.Releases[Point{bid, len(orig)}]...) {
		out = append(out, Inst{Op: OpRelease, Args: []LocalID{t}})
	}
	blk.Insts = out
}

func (p *Program) copyBinding(f *Func, in *Inst, st *rcState, w *shareFacts, at Point) []LocalID {
	if in.Op != OpCopy || in.Share || !w.copyAt[at] || f.Locals[st.origin(in.Args[0])].Ent != 0 {
		return nil
	}
	in.Op, in.Copy, in.IfWritable, in.Own = OpShare, true, p.R.WritableByParam(f.Locals[in.Args[0]].Type), true
	if w.liveness().after(in.Args[0], at.Block, at.Inst) {
		return nil
	}
	return []LocalID{in.Args[0]}
}

// rewriteCopy turns one OpCopy into OpAlias (no runtime call) or OpShare
// (rt_copy/rt_retain), following the same cases the backends' OpCopy
// handling does: an eager Share, a source that is itself a temporary or a
// borrow (a pure alias), a single-definition temporary destination
// deferred to its own consuming use, or an eager share by default.
func (p *Program) rewriteCopy(f *Func, in *Inst, plan *Plan, st *rcState, w *shareFacts, at Point) {
	switch in.Op {
	case OpBorrow:
		st.borrowed[in.Dst] = true
		return
	case OpNewCell:
		st.borrowed[in.Dst] = in.Lent
		return
	case OpAlias:
		// A defined-type wrap/unwrap emits this instead of an OpCopy PlanRC
		// rewrites, but its chain must still be recorded for origin() to work.
		if len(in.Args) == 1 {
			st.alias[in.Dst] = st.origin(in.Args[0])
		}
		return
	case OpCopy:
	default:
		return
	}
	src := st.origin(in.Args[0])
	switch {
	case in.Share:
		in.Op, in.Copy = OpShare, p.copyable(f.Locals[src].Type)
	case f.Locals[src].Ent == 0 || st.borrowed[in.Args[0]]:
		in.Op = OpAlias
	case f.Locals[in.Dst].Ent == 0 && plan.Defs[in.Dst] <= 1 && !plan.Materialize[in.Dst]:
		in.Op = OpAlias
		st.alias[in.Dst] = src
	default:
		in.Op, in.Copy = OpShare, p.copyable(f.Locals[src].Type)
		if in.Last = p.bindsLast(f, *in, src, w, at); in.Last {
			in.Args = []LocalID{src}
		}
		in.Retain = in.Retain && !in.Last && !w.copyBinding(at, *in)
	}
	in.Share = false
}

func (p *Program) shareReturn(f *Func, t *Term, st *rcState, out *[]Inst, w *shareFacts, at Point) []LocalID {
	if t.Op != TermReturn {
		return nil
	}
	a := t.Args[0]
	src := st.origin(a)
	if st.borrowed[a] {
		return nil
	}
	if f.Locals[src].Ent != 0 {
		nt := f.newTemp(f.Locals[a].Type)
		last := p.returnsLast(f, a, src, w, at)
		from := a
		if last {
			from = src
		}
		*out = append(*out, Inst{Op: OpShare, Dst: nt, Args: []LocalID{from}, Copy: p.copyable(f.Locals[src].Type), Node: w.defNode(a), Last: last})
		t.Args = []LocalID{nt}
		return nil
	}
	if !w.returnCopies(a) {
		return nil
	}
	nt := f.newTemp(f.Locals[a].Type)
	*out = append(*out, Inst{Op: OpShare, Dst: nt, Args: []LocalID{a}, Copy: true, IfWritable: p.R.WritableByParam(f.Locals[a].Type), Own: true, Node: w.defNode(a)})
	t.Args = []LocalID{nt}
	return []LocalID{a}
}

// newTemp appends a fresh temporary of type t, named like the builder's
// own temporaries so a golden dump reads the same way.
func (f *Func) newTemp(t sem.TypeID) LocalID {
	f.Locals = append(f.Locals, Local{Name: fmt.Sprintf("t%d", len(f.Locals)), Type: t})
	return LocalID(len(f.Locals) - 1)
}
