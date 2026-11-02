package build

import (
	"slices"

	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

func (b *builder) pushScope() { b.scopes = append(b.scopes, &scope{}) }

// exitScope lowers the scope's defers and drops on the fall-through path.
func (b *builder) exitScope() {
	s := b.scopes[len(b.scopes)-1]
	b.scopes = b.scopes[:len(b.scopes)-1]
	b.unwindScope(s, false)
	b.queueGhosts(s)
}

// unwindScope emits the cleanup of one scope: defers in reverse, then the
// drops of its owned locals in reverse declaration order. A defer body
// that moves one of those locals by value takes its final slot value, so
// the normal drop is skipped.
func (b *builder) unwindScope(s *scope, failing bool) {
	consumed := map[midir.LocalID]bool{}
	for i := len(s.defers) - 1; i >= 0; i-- {
		d := s.defers[i]
		if d.Err && !failing {
			continue
		}
		if d.Err {
			if s.lowered == nil {
				s.lowered = map[*hir.Stmt]bool{}
			}
			s.lowered[d] = true
		}
		b.noteCleanup(d.Node)
		fromBlock, fromInst := b.cur, len(b.block().Insts)
		if d.Block != nil {
			b.blockExpr(d.Block)
		} else {
			b.stmt(d.Body)
		}
		b.markMoved(fromBlock, fromInst, consumed)
	}
	b.dropOwnedFrom(s, 0, consumed)
}

func (b *builder) dropOwnedFrom(s *scope, lo int, consumed map[midir.LocalID]bool) {
	for i := len(s.owned) - 1; i >= lo; i-- {
		if consumed[s.owned[i]] {
			continue
		}
		l := s.owned[i]
		b.emit(midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{l}, Node: b.dropNode(l), EndScope: true})
	}
}

func (b *builder) noteCleanup(n syntax.NodeID) {
	if !slices.Contains(b.fn.Cleanups, n) {
		b.fn.Cleanups = append(b.fn.Cleanups, n)
	}
}

// dropNode is the node to blame for a synthesized end-of-scope drop of l: l's own
// declaration when it has one, else the enclosing function's (an unnamed temporary has none).
func (b *builder) dropNode(l midir.LocalID) syntax.NodeID {
	if ent := b.fn.Locals[l].Ent; ent != 0 {
		return b.r.Entity(ent).Node
	}
	return b.r.Entity(b.fn.Ent).Node
}

// markMoved records every local a just-lowered defer body (the
// instructions from fromBlock/fromInst through the current cursor) moves
// as a whole value, so unwindScope can skip its now-redundant drop.
func (b *builder) markMoved(fromBlock midir.BlockID, fromInst int, consumed map[midir.LocalID]bool) {
	for bid := fromBlock; bid <= b.cur; bid++ {
		insts := b.fn.Blocks[bid].Insts
		start := 0
		if bid == fromBlock {
			start = fromInst
		}
		for _, in := range insts[start:] {
			if in.Op == midir.OpMove {
				consumed[in.Args[0]] = true
			}
		}
	}
}

// unwindTo emits the cleanups of every scope down to depth (exclusive),
// for early exits.
func (b *builder) unwindTo(depth int, failing bool) {
	b.abandonFixups(depth)
	for i := len(b.scopes) - 1; i >= depth; i-- {
		b.unwindScope(b.scopes[i], failing)
	}
}

// own registers a temporary with the innermost scope so it is dropped like
// a binding.
func (b *builder) own(l midir.LocalID) midir.LocalID {
	s := b.scopes[len(b.scopes)-1]
	s.owned = append(s.owned, l)
	return l
}

// detach copies a named local into a temporary before the scope that owns
// the local is unwound, so the value returned outlives the drops.
func (b *builder) detach(v midir.LocalID) midir.LocalID {
	if b.fn.Locals[v].Ent == 0 {
		return v
	}
	return b.emit(midir.Inst{Op: midir.OpCopy, Dst: b.temp(b.fn.Locals[v].Type), Args: []midir.LocalID{v}})
}

// declare makes a local for a binding entity; locals a closure assigns
// through live in cells. Every binding is owned by its scope and dropped
// at exit: for Copy values the drop only frees the backend's storage.
func (b *builder) declare(ent sem.EntityID) midir.LocalID {
	e := b.r.Entity(ent)
	id := midir.LocalID(len(b.fn.Locals))
	b.fn.Locals = append(b.fn.Locals, midir.Local{Name: e.Name, Type: e.Type, Ent: ent, Cell: e.Flags&(sem.EfCapturedMut|sem.EfAddrTaken) != 0})
	b.locals[ent] = id
	refKind := b.r.Types.Kind(e.Type) == sem.KRef
	// A reference parameter is lent, not handed over; ownIfCounted overrides this once its binding instruction runs.
	b.unowned[id] = refKind
	if len(b.scopes) > 0 && !refKind {
		s := b.scopes[len(b.scopes)-1]
		s.owned = append(s.owned, id)
	}
	return id
}

// ownIfCounted marks a reference owned once its binding proves it counted; declare can't tell the two cases apart up front.
func (b *builder) ownIfCounted(id midir.LocalID) {
	if b.r.Types.Kind(b.fn.Locals[id].Type) != sem.KRef || b.unowned[id] {
		return
	}
	b.own(id)
}

// param declares an ordinary parameter. One a closure assigns arrives as
// a plain value and is moved into a cell, as a `let` binding would be.
func (b *builder) param(d *hir.LocalDecl) midir.LocalID {
	if b.r.Entity(d.Ent).Flags&(sem.EfCapturedMut|sem.EfAddrTaken) == 0 {
		return b.declare(d.Ent)
	}
	raw := b.temp(d.Type)
	id := b.declare(d.Ent)
	b.emit(midir.Inst{Op: midir.OpNewCell, Dst: id, Args: []midir.LocalID{raw}})
	return raw
}

// local returns the local of a binding, declaring parameters of the
// enclosing function lazily.
func (b *builder) local(ent sem.EntityID) midir.LocalID {
	if id, ok := b.locals[ent]; ok {
		return id
	}
	return b.declare(ent)
}
