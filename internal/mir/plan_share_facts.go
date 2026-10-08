package mir

import (
	"slices"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// shareFacts reads a body as it was before PlanRC rewrote it; Points name instructions of that snapshot.
type shareFacts struct {
	p       *Program
	f       *Func
	sum     *shareSummary
	blocks  []Block
	nLocals int
	defs    [][]*Inst
	binds   map[LocalID][]Point
	sets    map[LocalID][]Point
	uses    map[LocalID][]argUse
	own     map[LocalID][]Point
	inner   map[LocalID][]Point
	paths   map[rootAt][]wpath
	blind   []Point
	lent    []lentSite
	copyAt  map[Point]bool
	decided bool
	memo    map[LocalID]rootSet
	viewers map[LocalID][]LocalID
	reach   [][]bool
	live    *liveness
}

type argUse struct {
	at   Point
	i    int
	term bool
}

func newShareFacts(p *Program, f *Func, sum *shareSummary) *shareFacts {
	w := &shareFacts{p: p, f: f, sum: sum, blocks: slices.Clone(f.Blocks), nLocals: len(f.Locals), defs: make([][]*Inst, len(f.Locals)),
		binds: map[LocalID][]Point{}, sets: map[LocalID][]Point{}, uses: map[LocalID][]argUse{}, own: map[LocalID][]Point{}, inner: map[LocalID][]Point{},
		paths: map[rootAt][]wpath{}, copyAt: map[Point]bool{}}
	for b, blk := range w.blocks {
		for k := range blk.Insts {
			in := &blk.Insts[k]
			for i, a := range in.Args {
				w.uses[a] = append(w.uses[a], argUse{at: Point{b, k}, i: i})
			}
			switch {
			case !in.Op.Yields():
			case f.Locals[in.Dst].Ent == 0:
				w.defs[in.Dst] = append(w.defs[in.Dst], in)
			default:
				w.sets[in.Dst] = append(w.sets[in.Dst], Point{b, k})
				if bindsFrom(*in) && f.Locals[in.Args[0]].Ent == 0 {
					w.binds[in.Dst] = append(w.binds[in.Dst], Point{b, k})
				}
			}
		}
		for i, a := range blk.Term.Args {
			w.uses[a] = append(w.uses[a], argUse{at: Point{b, len(blk.Insts)}, i: i, term: true})
		}
	}
	for b, blk := range w.blocks {
		for k, in := range blk.Insts {
			w.noteWrites(in, Point{b, k})
		}
	}
	return w
}

func (w *shareFacts) inst(at Point) Inst { return w.blocks[at.Block].Insts[at.Inst] }

func (w *shareFacts) defNode(v LocalID) syntax.NodeID {
	for _, in := range w.defs[v] {
		if in.Node != 0 {
			return in.Node
		}
	}
	return 0
}

func bindsFrom(in Inst) bool {
	switch in.Op {
	case OpCopy:
		return !in.Share && len(in.Args) == 1
	case OpMove, OpAlias:
		return len(in.Args) == 1
	}
	return false
}

func (w *shareFacts) noteWrites(in Inst, at Point) {
	switch in.Op {
	case OpSetField, OpSetIndex:
		w.noteWrite(in.Args[0], at, false)
	case OpBorrow:
		if in.Index == 1 {
			to, ok := w.lentTo(in.Dst)
			w.lendOrWrite(in.Args[0], at, to, ok)
		}
	case OpCall, OpCallValue, OpCallC, OpFutureNew:
		for i, a := range in.Args {
			if w.passesPlace(a) {
				to, ok := w.calleeParam(in, i)
				w.lendOrWrite(a, at, to, ok && w.p.R.Types.Kind(w.f.Locals[a].Type) == sem.KRef)
			}
		}
	}
}

func (w *shareFacts) passesPlace(a LocalID) bool {
	l := w.f.Locals[a]
	return l.Ent != 0 && (l.Cell || w.mutRef(a))
}

// A callee handed a place may write anything inside it, so whole files the write under both.
func (w *shareFacts) noteWrite(base LocalID, at Point, whole bool) {
	r, path, ok := w.placeOf(base)
	if !ok {
		w.blind = append(w.blind, at)
		return
	}
	if len(path) == 0 {
		w.own[r] = append(w.own[r], at)
	}
	if len(path) > 0 || whole {
		w.inner[r] = append(w.inner[r], at)
		w.paths[rootAt{r, at}] = append(w.paths[rootAt{r, at}], wpath{path, whole})
	}
}

func projects(in Inst) bool {
	switch in.Op {
	case OpField, OpIndex, OpPayload, OpUnbox:
		return true
	case OpBuiltin:
		return in.Str == "iter.at" || in.Str == "iter.at_ref"
	}
	return false
}

func passesOn(op Opcode) bool {
	switch op {
	case OpCopy, OpAlias, OpMove, OpBorrow, OpCellGet, OpNewCell, OpShare:
		return true
	}
	return false
}

func (w *shareFacts) liveness() *liveness {
	if w.live == nil {
		w.live = newLiveness(w.blocks, w.nLocals)
	}
	return w.live
}

func (w *shareFacts) reachable(from Point, sites []Point) bool {
	for _, s := range sites {
		if s.Block == from.Block && s.Inst > from.Inst {
			return true
		}
		if w.reach == nil {
			w.reach = blockReach(w.blocks)
		}
		if w.reach[from.Block][s.Block] {
			return true
		}
	}
	return false
}
