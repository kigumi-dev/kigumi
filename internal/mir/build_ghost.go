package mir

import "kigumi/internal/hir"

// exitMark is a place in a scope where a success exit starts, before the exit's value moves anything; defers is how many cleanups were registered by then.
type exitMark struct {
	pos    markPos
	defers int
}

type markPos struct {
	blk BlockID
	idx int
}

// pendingGhost is a defer(fail) no failing exit lowers.
type pendingGhost struct {
	stmt  *hir.Stmt
	marks []markPos
}

// ghostRun is a lowered pendingGhost: blocks [entry, end) hold the body, reachable only through the analysis.
type ghostRun struct {
	entry, end BlockID
	marks      []markPos
}

type ghostSet struct {
	runs   []ghostRun
	blocks int
	locals int
}

func (s *scope) hasFailDefer() bool {
	for _, d := range s.defers {
		if d.Err {
			return true
		}
	}
	return false
}

func (b *builder) markExit(depth int) {
	if b.block().Term.Op != TermNone {
		return
	}
	for _, s := range b.scopes[depth:] {
		if s.hasFailDefer() {
			s.marks = append(s.marks, exitMark{pos: markPos{b.cur, len(b.block().Insts)}, defers: len(s.defers)})
		}
	}
}

func (b *builder) queueGhosts(s *scope) {
	for i, d := range s.defers {
		if !d.Err || s.lowered[d] {
			continue
		}
		g := pendingGhost{stmt: d}
		for _, m := range s.marks {
			if m.defers > i {
				g.marks = append(g.marks, m.pos)
			}
		}
		if len(g.marks) > 0 {
			b.ghosts = append(b.ghosts, g)
		}
	}
}

// lowerGhosts appends each queued body after the real blocks and ends it unreachable, so the ownership analysis checks it and dropGhosts removes it.
func (b *builder) lowerGhosts() {
	if len(b.ghosts) == 0 {
		return
	}
	b.fn.ghosts.blocks, b.fn.ghosts.locals = len(b.fn.Blocks), len(b.fn.Locals)
	for _, g := range b.ghosts {
		run := ghostRun{entry: b.newBlock(), marks: g.marks}
		b.pushScope()
		b.noteCleanup(g.stmt.Node)
		if g.stmt.Block != nil {
			b.blockExpr(g.stmt.Block)
		} else {
			b.stmt(g.stmt.Body)
		}
		b.scopes = b.scopes[:len(b.scopes)-1]
		b.term(Term{Op: TermUnreachable})
		run.end = BlockID(len(b.fn.Blocks))
		b.fn.ghosts.runs = append(b.fn.ghosts.runs, run)
	}
}

func (f *Func) dropGhosts() {
	if len(f.ghosts.runs) == 0 {
		return
	}
	f.Blocks, f.Locals = f.Blocks[:f.ghosts.blocks], f.Locals[:f.ghosts.locals]
	f.ghosts = ghostSet{}
}
