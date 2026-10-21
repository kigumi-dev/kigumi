package mir

import (
	"strconv"
	"strings"

	"kigumi/internal/diag"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

type movePos struct {
	bid BlockID
	idx int
}

// moveEvent is a move, consume or in-place change of root's value; field marks a field move checkCapturedFieldMoves already sees while live.
type moveEvent struct {
	at    movePos
	root  LocalID
	node  syntax.NodeID
	field bool
}

func (a *owner) noteMove(in *Inst, l LocalID, report bool) {
	if !report {
		return
	}
	root := l
	if k := a.pl.key[l]; k != "" {
		id, err := strconv.Atoi(strings.SplitN(k, ".", 2)[0])
		if err != nil {
			return
		}
		root = LocalID(id)
	}
	a.moves = append(a.moves, moveEvent{at: a.pos, root: root, node: in.Node})
}

// fieldMoved: binding a `move` of a field is the field move itself, already its own event.
func (a *owner) fieldMoved(l LocalID) bool {
	if a.fieldMv == nil {
		a.fieldMv = map[LocalID]bool{}
		for _, blk := range a.f.Blocks {
			for _, in := range blk.Insts {
				if in.Op == OpFieldMove {
					a.fieldMv[in.Dst] = true
				}
			}
		}
	}
	return a.fieldMv[l]
}

type readCheck struct {
	p      *Program
	f      *Func
	defs   map[LocalID]*Inst
	events map[BlockID][]moveEvent
	seen   map[syntax.NodeID]bool
}

// checkReadCaptures: a closure that only reads a move-only local shares it, so the local is neither moved nor changed while the closure may run.
func checkReadCaptures(p *Program, f *Func, moves []moveEvent) {
	c := &readCheck{p: p, f: f, defs: soleDefs(f), events: map[BlockID][]moveEvent{}, seen: map[syntax.NodeID]bool{}}
	caps := holdersFrom(p, f, c.readCaptures)
	if len(caps) == 0 {
		return
	}
	for _, m := range append(moves, c.changes()...) {
		c.events[m.at.bid] = append(c.events[m.at.bid], m)
	}
	liveOut := capturedLiveness(f, caps)
	for bi := range f.Blocks {
		blk := &f.Blocks[bi]
		live := map[LocalID]bool{}
		for l := range liveOut[bi] {
			live[l] = true
		}
		markCapturedUses(blk.Term.Args, caps, live)
		for i := len(blk.Insts) - 1; i >= 0; i-- {
			in := &blk.Insts[i]
			for _, m := range c.events[BlockID(bi)] {
				if m.at.idx == i && !m.field && sharedBy(capturedPlace{root: m.root}, caps, live) {
					c.report(m.node, m.root)
				}
			}
			if in.Op == OpDrop {
				continue
			}
			for _, h := range c.escapes(*in, caps) {
				c.walkAfter(movePos{BlockID(bi), i}, caps[h])
			}
			delete(live, in.Dst)
			markCapturedUses(in.Args, caps, live)
		}
	}
}

func (c *readCheck) readCaptures(in Inst) []capturedPlace {
	if in.Op != OpClosure || c.p.R.Entity(in.Ent).Kind != sem.EntClosure {
		return nil
	}
	var out []capturedPlace
	for ci, cp := range c.p.R.Closure(in.Ent).Captures {
		if cp.Mode != sem.CapRead || ci >= len(in.Args) {
			continue
		}
		if r := c.root(in.Args[ci]); c.f.Locals[r].Ent != 0 {
			out = append(out, capturedPlace{root: r})
		}
	}
	return out
}

// walkAfter checks everything reachable after an escape, where the closure may run at any time.
func (c *readCheck) walkAfter(from movePos, held []capturedPlace) {
	visited := map[BlockID]bool{}
	var walk func(bid BlockID, after int)
	walk = func(bid BlockID, after int) {
		for _, m := range c.events[bid] {
			if m.at.idx > after && hasCapturedPlace(held, capturedPlace{root: m.root}) {
				c.report(m.node, m.root)
			}
		}
		for _, t := range c.f.Blocks[bid].Term.Targets {
			if !visited[t] {
				visited[t] = true
				walk(t, -1)
			}
		}
	}
	walk(from.bid, from.idx)
}

func (c *readCheck) report(n syntax.NodeID, root LocalID) {
	if n == 0 || c.seen[n] {
		return
	}
	c.seen[n] = true
	t := c.p.R.Tree(c.f.File)
	msg := "`" + c.f.Locals[root].Name + "` is shared with a closure that may still run, so it cannot be moved or changed here"
	c.f.Diags = append(c.f.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Code: "move-while-borrowed", Msg: msg})
}
