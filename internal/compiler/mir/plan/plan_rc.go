package plan

import (
	"cmp"
	"fmt"
	"slices"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// After RC a backend translates opcodes one by one without consulting Plan.
func RC(p *mir.Program) error { return (&planner{Program: p}).planRC() }

func (p *planner) planRC() error {
	fs := p.AllFuncs()
	s := p.planShares(fs)
	for _, f := range fs {
		p.planRCFunc(f, s.facts[f])
		p.markValueArgs(f)
	}
	return planAsync(p)
}

// rcState maps each temporary to the named local it stands for and marks borrow results, which are never copied.
type rcState struct {
	alias    map[midir.LocalID]midir.LocalID
	borrowed map[midir.LocalID]bool
	// viaBorrow marks a local aliasing a borrow; it holds no count either.
	viaBorrow map[midir.LocalID]bool
}

func (s *rcState) origin(l midir.LocalID) midir.LocalID {
	for {
		src, ok := s.alias[l]
		if !ok {
			return l
		}
		l = src
	}
}

func (p *planner) planRCFunc(f *midir.Func, w *shareFacts) {
	f.RCBase = len(f.Locals)
	plan := p.planFunc(f)
	st := &rcState{alias: map[midir.LocalID]midir.LocalID{}, borrowed: map[midir.LocalID]bool{}, viaBorrow: map[midir.LocalID]bool{}}
	for t := range plan.LentJoins {
		st.viaBorrow[t] = true
	}
	for bid := range f.Blocks {
		p.planRCBlock(f, bid, plan, st, w)
	}
	splitEdges(f, plan)
}

func splitEdges(f *midir.Func, plan *Plan) {
	edges := make([]mir.Edge, 0, len(plan.Splits))
	for e := range plan.Splits {
		edges = append(edges, e)
	}
	slices.SortFunc(edges, func(a, b mir.Edge) int { return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.Slot, b.Slot)) })
	for _, e := range edges {
		var insts []midir.Inst
		for _, t := range plan.Splits[e] {
			insts = append(insts, midir.Inst{Op: midir.OpRelease, Args: []midir.LocalID{t}})
		}
		to := f.Blocks[e.From].Term.Targets[e.Slot]
		f.Blocks = append(f.Blocks, midir.Block{Insts: insts, Term: midir.Term{Op: midir.TermJump, Targets: []midir.BlockID{to}}})
		f.Blocks[e.From].Term.Targets[e.Slot] = midir.BlockID(len(f.Blocks) - 1)
	}
}

func (p *planner) planRCBlock(f *midir.Func, bid int, plan *Plan, st *rcState, w *shareFacts) {
	blk := &f.Blocks[bid]
	orig := blk.Insts
	out := make([]midir.Inst, 0, len(orig))
	for _, t := range plan.Releases[Point{bid, -1}] {
		out = append(out, midir.Inst{Op: midir.OpRelease, Args: []midir.LocalID{t}})
	}
	for k := range orig {
		in, at := orig[k], Point{bid, k}
		release := p.shareArgs(f, &in, st, &out, w, at)
		release = append(release, p.copyBinding(f, &in, st, w, at)...)
		p.rewriteCopy(f, &in, plan, st, w, at)
		out = append(out, in)
		for _, t := range append(release, plan.Releases[at]...) {
			out = append(out, midir.Inst{Op: midir.OpRelease, Args: []midir.LocalID{t}})
		}
	}
	release := p.shareReturn(f, &blk.Term, st, &out, w, Point{bid, len(orig)})
	for _, t := range append(release, plan.Releases[Point{bid, len(orig)}]...) {
		out = append(out, midir.Inst{Op: midir.OpRelease, Args: []midir.LocalID{t}})
	}
	blk.Insts = out
}

func (p *planner) copyBinding(f *midir.Func, in *midir.Inst, st *rcState, w *shareFacts, at Point) []midir.LocalID {
	if in.Op != midir.OpCopy || in.Share || !w.copyAt[at] || f.Locals[st.origin(in.Args[0])].Ent != 0 {
		return nil
	}
	in.Op, in.Copy, in.IfWritable, in.Own = midir.OpShare, true, p.R.WritableByParam(f.Locals[in.Args[0]].Type), true
	if w.liveness().after(in.Args[0], at.Block, at.Inst) {
		return nil
	}
	return []midir.LocalID{in.Args[0]}
}

// rewriteCopy turns one OpCopy into OpAlias (no runtime call) or OpShare (rt_copy/rt_retain).
func (p *planner) rewriteCopy(f *midir.Func, in *midir.Inst, plan *Plan, st *rcState, w *shareFacts, at Point) {
	switch in.Op {
	case midir.OpBorrow, midir.OpLendBox:
		st.borrowed[in.Dst] = true
		return
	case midir.OpNewCell:
		st.borrowed[in.Dst] = in.Lent
		return
	case midir.OpAlias:
		// A defined-type wrap or unwrap emits this, and origin() still needs its chain.
		if len(in.Args) == 1 {
			st.alias[in.Dst] = st.origin(in.Args[0])
		}
		return
	case midir.OpCopy:
	default:
		return
	}
	src := st.origin(in.Args[0])
	switch {
	case plan.LentJoins[in.Dst]:
		in.Op = midir.OpAlias
	case in.Share:
		in.Op, in.Copy = midir.OpShare, p.copyable(f.Locals[src].Type)
	case (st.borrowed[in.Args[0]] || st.viaBorrow[in.Args[0]]) && plan.Defs[in.Dst] > 1 && returned(f, in.Dst):
		in.Op, in.Copy = midir.OpShare, p.copyable(f.Locals[in.Dst].Type)
	case f.Locals[src].Ent == 0 || st.borrowed[in.Args[0]]:
		in.Op = midir.OpAlias
		st.viaBorrow[in.Dst] = (f.Locals[in.Dst].Ent == 0 || plan.Defs[in.Dst] <= 1) && (st.borrowed[in.Args[0]] || st.viaBorrow[in.Args[0]])
	case f.Locals[in.Dst].Ent == 0 && plan.Defs[in.Dst] <= 1 && !plan.Materialize[in.Dst]:
		in.Op = midir.OpAlias
		st.alias[in.Dst] = src
	case plan.Defs[in.Dst] <= 1 && p.countless(f, st, src):
		in.Op = midir.OpAlias
		st.viaBorrow[in.Dst] = true
	default:
		in.Op, in.Copy = midir.OpShare, p.copyable(f.Locals[src].Type) && !w.bindingShares(at)
		if in.Last = p.bindsLast(f, *in, src, w, at); in.Last {
			in.Args = []midir.LocalID{src}
		}
		in.Retain = in.Retain && !in.Last && !w.copyBinding(at, *in)
	}
	in.Share = false
}

func (p *planner) shareReturn(f *midir.Func, t *midir.Term, st *rcState, out *[]midir.Inst, w *shareFacts, at Point) []midir.LocalID {
	if t.Op != midir.TermReturn {
		return nil
	}
	a := t.Args[0]
	src := st.origin(a)
	if st.borrowed[a] || st.viaBorrow[a] {
		if p.R.Types.Kind(f.Locals[a].Type) != sem.KRef {
			return nil
		}
		nt := newTemp(f, f.Locals[a].Type)
		*out = append(*out, midir.Inst{Op: midir.OpShare, Dst: nt, Args: []midir.LocalID{a}, Copy: p.copyable(f.Locals[a].Type), Node: w.defNode(a)})
		t.Args = []midir.LocalID{nt}
		return nil
	}
	if f.Locals[src].Ent != 0 {
		nt := newTemp(f, f.Locals[a].Type)
		last := p.returnsLast(f, a, src, w, at)
		from := a
		if last {
			from = src
		}
		*out = append(*out, midir.Inst{Op: midir.OpShare, Dst: nt, Args: []midir.LocalID{from}, Copy: p.copyable(f.Locals[src].Type), Node: w.defNode(a), Last: last})
		t.Args = []midir.LocalID{nt}
		return nil
	}
	if !w.returnCopies(a) {
		return nil
	}
	nt := newTemp(f, f.Locals[a].Type)
	*out = append(*out, midir.Inst{Op: midir.OpShare, Dst: nt, Args: []midir.LocalID{a}, Copy: true, IfWritable: p.R.WritableByParam(f.Locals[a].Type), Own: true, Node: w.defNode(a)})
	t.Args = []midir.LocalID{nt}
	return []midir.LocalID{a}
}

// newTemp appends a fresh temporary of type t, named like the builder's
// own temporaries so a golden dump reads the same way.
func newTemp(f *midir.Func, t sem.TypeID) midir.LocalID {
	f.Locals = append(f.Locals, midir.Local{Name: fmt.Sprintf("t%d", len(f.Locals)), Type: t})
	return midir.LocalID(len(f.Locals) - 1)
}
