package mir

import (
	"slices"

	"kigumi/internal/sem"
)

func (w *shareFacts) sinkShared(in Inst, i int, at Point) bool {
	a := in.Args[i]
	if w.shared(a, at) {
		return !w.movedOut(in, i, at) && !w.leftBehind(in, i, at)
	}
	if !w.writable(a) {
		return false
	}
	for j, b := range in.Args {
		if j == i || w.p.R.Types.Kind(w.f.Locals[b].Type) == sem.KPrim || w.p.sinkAt(w.f, in, j, w.sum) == noSink || w.copiedAnyway(in, j, at) {
			continue
		}
		if d, ok := w.overlap(a, b, at); ok && (d < 0 || d == 0 && j < i) {
			return true
		}
	}
	return false
}

// overlap compares b's depth with a's, so the outermost read keeps the storage and the others copy.
func (w *shareFacts) overlap(a, b LocalID, at Point) (int, bool) {
	if a == b {
		return 0, true
	}
	ra, pa, oka := w.placeOf(a)
	rb, pb, okb := w.placeOf(b)
	if oka && okb && ra == rb && (len(w.sets[ra]) <= 1 || w.readSince(a, ra, at) && w.readSince(b, ra, at)) {
		n := min(len(pa), len(pb))
		return len(pb) - len(pa), slices.Equal(pa[:n], pb[:n])
	}
	for _, x := range w.rootsOf(a).refs {
		for _, y := range w.rootsOf(b).refs {
			if x.l == y.l {
				return 0, true
			}
		}
	}
	return 0, false
}

func (w *shareFacts) readSince(v, r LocalID, at Point) bool {
	head := v
	for head != r {
		next := w.defs[head][0].Args[0]
		if next == r {
			break
		}
		head = next
	}
	if head == r {
		return true
	}
	for k := at.Inst - 1; k >= 0; k-- {
		switch in := w.blocks[at.Block].Insts[k]; {
		case !in.Op.Yields():
		case in.Dst == head:
			return true
		case in.Dst == r:
			return false
		}
	}
	return false
}

func (w *shareFacts) copiedAnyway(in Inst, j int, at Point) bool {
	o, ok := w.namedOrigin(in.Args[j])
	if !ok || w.p.inPlace(in, j) || !w.p.mustCopy(w.f, in, o) {
		return false
	}
	return !(w.p.movable(w.f, o) && w.ownsBox(o) && !w.observed(o, at))
}
