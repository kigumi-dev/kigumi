package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

func (e *erasedReaders) lentReaders() {
	owners := map[sem.EntityID]midir.LocalID{}
	for l, loc := range e.f.Locals {
		if _, seen := owners[loc.Ent]; loc.Ent != 0 && !seen {
			owners[loc.Ent] = midir.LocalID(l)
		}
	}
	for bi := range e.f.Blocks {
		for i, in := range e.f.Blocks[bi].Insts {
			if in.Op != midir.OpCall && in.Op != midir.OpCallValue {
				continue
			}
			at := movePos{midir.BlockID(bi), i}
			for k, a := range in.Args {
				e.bc.curBlock, e.bc.curIdx = at.bid, at.idx
				for _, r := range e.lentReads(in, k, owners) {
					if r.shared {
						if n, ok := e.earlierWriter(at, r); ok {
							e.noteLent(in, a, erasedWrite{write: n, root: r.o.owner, how: lentAfterMade})
							break
						}
					}
					e.bc.curBlock, e.bc.curIdx = at.bid, at.idx
					if b, ok := e.besideWriter(in, a, r.o); ok {
						e.noteLent(in, a, erasedWrite{write: e.argNode(b, in), root: r.o.owner, how: lentBeside})
						break
					}
				}
			}
		}
	}
}

type lentRead struct {
	cl     sem.EntityID
	o      ownerRef
	shared bool
}

func lentReach(m sem.CaptureMode) bool { return storageReach(m) || m == sem.CapCopy }

func (e *erasedReaders) lentReads(in midir.Inst, k int, owners map[sem.EntityID]midir.LocalID) []lentRead {
	if !e.erasedParam(in, k) {
		return nil
	}
	var out []lentRead
	for _, cl := range e.bc.argClosures(in.Args[k]) {
		for idx, cp := range e.p.R.Closure(cl).Captures {
			r, ok := owners[cp.Local]
			if !lentReach(cp.Mode) || !ok {
				continue
			}
			for _, u := range e.bc.closureUses(cl, idx) {
				out = append(out, lentRead{cl: cl, o: ownerRef{owner: r, path: u.path}, shared: storageReach(cp.Mode)})
			}
		}
	}
	return out
}

func (e *erasedReaders) noteLent(in midir.Inst, a midir.LocalID, w erasedWrite) {
	if e.lentReported(w) {
		return
	}
	at := e.site[a]
	if at == 0 {
		at = e.argNode(a, in)
	}
	if _, ok := e.found[at]; !ok {
		e.found[at] = w
	}
}

func (e *erasedReaders) besideWriter(in midir.Inst, a midir.LocalID, o ownerRef) (midir.LocalID, bool) {
	e.windowInit()
	for k, b := range in.Args {
		if b == a {
			continue
		}
		if slices.ContainsFunc(e.argWrites(in, k, o.owner), func(q stepPath) bool { return fieldPathsOverlap(q, o.path) }) {
			return b, true
		}
		if hit, _ := e.bc.runHitVia(in, o, func(l midir.LocalID, _ sem.EntityID) bool { return l != b }, storageReach); hit {
			return b, true
		}
	}
	return 0, false
}

// erasedParam reports an argument whose parameter type holds plain `fn` values (a variadic pack among them).
func (e *erasedReaders) erasedParam(in midir.Inst, k int) bool {
	if in.Op == midir.OpCallValue && k == 0 {
		return false
	}
	t, known := e.bc.callParam(e.f, in, k)
	return known && e.bc.typeHolds(t, func(t sem.TypeID) bool { return e.p.R.Types.Kind(t) == sem.KFn })
}

func (e *erasedReaders) argNode(l midir.LocalID, in midir.Inst) syntax.NodeID {
	if d := e.defs[l]; d != nil && d.Node != 0 {
		return d.Node
	}
	return in.Node
}
