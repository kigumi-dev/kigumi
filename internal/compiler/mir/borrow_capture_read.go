package mir

import (
	"strconv"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
)

type movePos struct {
	bid midir.BlockID
	idx int
}

// moveEvent is a move, consume or in-place change of root's value; field marks a field move checkCapturedFieldMoves already sees while live.
type moveEvent struct {
	at    movePos
	root  midir.LocalID
	node  syntax.NodeID
	field bool
}

func (a *owner) noteMove(in *midir.Inst, l midir.LocalID, report bool) {
	if !report {
		return
	}
	root := l
	if k := a.pl.key[l]; k != "" {
		id, err := strconv.Atoi(strings.SplitN(k, ".", 2)[0])
		if err != nil {
			return
		}
		root = midir.LocalID(id)
	}
	a.moves = append(a.moves, moveEvent{at: a.pos, root: root, node: in.Node})
}

// fieldMoved: binding a `move` of a field is the field move itself, already its own event.
func (a *owner) fieldMoved(l midir.LocalID) bool {
	if a.fieldMv == nil {
		a.fieldMv = map[midir.LocalID]bool{}
		for _, blk := range a.f.Blocks {
			for _, in := range blk.Insts {
				if in.Op == midir.OpFieldMove {
					a.fieldMv[in.Dst] = true
				}
			}
		}
	}
	return a.fieldMv[l]
}

type readCheck struct {
	p      *Program
	f      *midir.Func
	defs   map[midir.LocalID]*midir.Inst
	events map[midir.BlockID][]moveEvent
	seen   map[syntax.NodeID]bool
	// emit reports an event on root while holder may still run; dropUse names holders whose drop may run them.
	emit    func(n syntax.NodeID, root, holder midir.LocalID)
	dropUse func(midir.LocalID) bool
	// escapeFn replaces escapes for a check that follows more flows itself.
	escapeFn func(midir.Inst, map[midir.LocalID][]capturedPlace) []midir.LocalID
}

func newReadCheck(p *Program, f *midir.Func) *readCheck {
	return &readCheck{p: p, f: f, defs: soleDefs(f), events: map[midir.BlockID][]moveEvent{}, seen: map[syntax.NodeID]bool{}}
}

// checkReadCaptures: a closure that only reads a move-only local shares it, so the local is neither moved nor changed while the closure may run.
func checkReadCaptures(p *Program, f *midir.Func, moves []moveEvent) {
	c := newReadCheck(p, f)
	c.emit = c.report
	caps := holdersFrom(p, f, c.readCaptures)
	if len(caps) == 0 {
		return
	}
	c.scan(caps, append(moves, c.changes()...))
}

// scan reports each event on a place a holder shares while the holder is live, or at any point after it escaped.
func (c *readCheck) scan(caps map[midir.LocalID][]capturedPlace, events []moveEvent) {
	for _, m := range events {
		c.events[m.at.bid] = append(c.events[m.at.bid], m)
	}
	liveOut := capturedLiveness(c.f, caps, c.dropUse)
	for bi := range c.f.Blocks {
		blk := &c.f.Blocks[bi]
		live := map[midir.LocalID]bool{}
		for l := range liveOut[bi] {
			live[l] = true
		}
		markCapturedUses(blk.Term.Args, caps, live)
		for i := len(blk.Insts) - 1; i >= 0; i-- {
			in := &blk.Insts[i]
			for _, m := range c.events[midir.BlockID(bi)] {
				if h, ok := sharerOf(capturedPlace{root: m.root}, caps, live); m.at.idx == i && !m.field && ok {
					c.emit(m.node, m.root, h)
				}
			}
			if in.Op == midir.OpDrop {
				markDropUse(*in, caps, live, c.dropUse)
				continue
			}
			escapes := c.escapes
			if c.escapeFn != nil {
				escapes = c.escapeFn
			}
			for _, h := range escapes(*in, caps) {
				c.walkAfter(movePos{midir.BlockID(bi), i}, h, caps[h])
			}
			if in.Op.Yields() {
				delete(live, in.Dst)
			}
			markCapturedUses(in.Args, caps, live)
		}
	}
}

func (c *readCheck) readCaptures(in midir.Inst) []capturedPlace {
	if in.Op != midir.OpClosure || c.p.R.Entity(in.Ent).Kind != sem.EntClosure {
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
func (c *readCheck) walkAfter(from movePos, holder midir.LocalID, held []capturedPlace) {
	visited := map[midir.BlockID]bool{}
	var walk func(bid midir.BlockID, after int)
	walk = func(bid midir.BlockID, after int) {
		for _, m := range c.events[bid] {
			if m.at.idx > after && hasCapturedPlace(held, capturedPlace{root: m.root}) {
				c.emit(m.node, m.root, holder)
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

func (c *readCheck) report(n syntax.NodeID, root, _ midir.LocalID) {
	if n == 0 || c.seen[n] {
		return
	}
	c.seen[n] = true
	t := c.p.R.Tree(c.f.File)
	msg := "`" + c.f.Locals[root].Name + "` is shared with a closure that may still run, so it cannot be moved or changed here"
	c.f.Diags = append(c.f.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Code: "move-while-borrowed", Msg: msg})
}
