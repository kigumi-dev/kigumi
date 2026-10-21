package mir

import (
	"slices"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// cancelEdge unwinds like an early return, so failure-only cleanups stay skipped.
func (b *builder) cancelEdge(n syntax.NodeID) {
	cancelled := b.emit(Inst{Op: OpCancelled, Dst: b.temp(sem.TyBool), Node: n})
	cancel, cont := b.newBlockAt(), b.newBlockAt()
	b.term(Term{Op: TermBranch, Args: []LocalID{cancelled}, Targets: []BlockID{cancel, cont}})
	b.cur = cancel
	if prev := b.reusableCancel(); prev != nil {
		b.dropOwnedFrom(b.scopes[len(b.scopes)-1], prev.owned[len(prev.owned)-1], nil)
		b.jump(prev.entry)
	} else {
		b.unwindTo(0, false)
		b.emit(Inst{Op: OpCancelDone, Node: n})
		b.term(Term{Op: TermUnreachable})
	}
	b.noteCancel(cancel)
	b.cur = cont
}

// cancelPath is the cleanup of the last cancel edge: a later edge in the same scopes that only owns more locals drops those and joins it instead of repeating every drop.
type cancelPath struct {
	fn     *Func
	scopes []*scope
	owned  []int
	entry  BlockID
}

func (b *builder) cancelShape() (scopes []*scope, owned []int, ok bool) {
	if len(b.argFixups) > 0 {
		return nil, nil, false
	}
	for _, s := range b.scopes {
		if len(s.defers) > 0 {
			return nil, nil, false
		}
		owned = append(owned, len(s.owned))
	}
	return slices.Clone(b.scopes), owned, true
}

func (b *builder) reusableCancel() *cancelPath {
	prev := b.cancelled
	if prev == nil || prev.fn != b.fn {
		return nil
	}
	scopes, owned, ok := b.cancelShape()
	if !ok || !slices.Equal(scopes, prev.scopes) {
		return nil
	}
	last := len(owned) - 1
	if last < 0 || !slices.Equal(owned[:last], prev.owned[:last]) || owned[last] < prev.owned[last] {
		return nil
	}
	return prev
}

func (b *builder) noteCancel(entry BlockID) {
	scopes, owned, ok := b.cancelShape()
	if !ok {
		b.cancelled = nil
		return
	}
	b.cancelled = &cancelPath{fn: b.fn, scopes: scopes, owned: owned, entry: entry}
}
