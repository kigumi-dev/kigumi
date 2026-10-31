package mir

import (
	"slices"

	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
)

// exitMark is a place in a scope where a success exit starts, before the exit's value moves anything; defers is how many cleanups were registered by then.
type exitMark struct {
	pos    markPos
	defers int
}

type markPos struct {
	blk midir.BlockID
	idx int
}

// pendingGhost is a defer(fail) no failing exit lowers.
type pendingGhost struct {
	stmt  *hir.Stmt
	marks []markPos
}

// ghostRun is a lowered pendingGhost: blocks [entry, end) hold the body, reachable only through the analysis.
type ghostRun struct {
	entry, end midir.BlockID
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
	if b.block().Term.Op != midir.TermNone {
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

// lowerGhosts appends each queued body after the real blocks and ends it unreachable, so the ownership analysis checks it and drop removes it.
func (b *builder) lowerGhosts() {
	if len(b.ghosts) == 0 {
		return
	}
	b.ghostRuns.blocks, b.ghostRuns.locals = len(b.fn.Blocks), len(b.fn.Locals)
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
		b.term(midir.Term{Op: midir.TermUnreachable})
		run.end = midir.BlockID(len(b.fn.Blocks))
		b.ghostRuns.runs = append(b.ghostRuns.runs, run)
	}
}

// drop removes the ghost runs' blocks and locals; a block appended after them, as an edge split, stays.
func (g *ghostSet) drop(f *midir.Func) {
	if len(g.runs) == 0 {
		return
	}
	start, end := g.ghostRange(f)
	f.Blocks = slices.Delete(f.Blocks, start, end)
	for b := range f.Blocks {
		for i, t := range f.Blocks[b].Term.Targets {
			if int(t) >= end {
				f.Blocks[b].Term.Targets[i] = t - midir.BlockID(end-start)
			}
		}
	}
	f.Locals = f.Locals[:g.locals]
	*g = ghostSet{}
}
