package mir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// lentReaders reports a plain `fn` argument reading a cell that is written after the closure was made and before the
// call is done: by an instruction evaluated before the call, or by another argument the call runs.
func (e *erasedReaders) lentReaders() {
	cells := map[sem.EntityID]LocalID{}
	for l, loc := range e.f.Locals {
		if _, seen := cells[loc.Ent]; loc.Cell && loc.Ent != 0 && !seen {
			cells[loc.Ent] = LocalID(l)
		}
	}
	for bi := range e.f.Blocks {
		for i, in := range e.f.Blocks[bi].Insts {
			if in.Op != OpCall && in.Op != OpCallValue {
				continue
			}
			at := movePos{BlockID(bi), i}
			for k, a := range in.Args {
				e.bc.curBlock, e.bc.curIdx = at.bid, at.idx
				for _, r := range e.lentReads(in, k, cells) {
					if n, ok := e.earlierWriter(at, r); ok {
						e.noteLent(in, a, erasedWrite{write: n, root: r.o.owner, how: lentAfterMade})
						break
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

// lentRead is a cell place a plain `fn` argument may read through closure cl.
type lentRead struct {
	cl sem.EntityID
	o  ownerRef
}

// lentReads are the cell places argument k may read through the closures it carries, when the call takes it as plain
// `fn` values.
func (e *erasedReaders) lentReads(in Inst, k int, cells map[sem.EntityID]LocalID) []lentRead {
	if !e.erasedParam(in, k) {
		return nil
	}
	var out []lentRead
	for _, cl := range e.bc.argClosures(in.Args[k]) {
		for idx, cp := range e.p.R.Closure(cl).Captures {
			r, ok := cells[cp.Local]
			if !storageReach(cp.Mode) || !ok {
				continue
			}
			for _, u := range e.bc.closureUses(cl, idx) {
				out = append(out, lentRead{cl: cl, o: ownerRef{owner: r, path: u.path}})
			}
		}
	}
	return out
}

func (e *erasedReaders) noteLent(in Inst, a LocalID, w erasedWrite) {
	at := e.site[a]
	if at == 0 {
		at = e.argNode(a, in)
	}
	if _, ok := e.found[at]; !ok {
		e.found[at] = w
	}
}

// besideWriter is an argument of in other than a that may run a closure writing o's place.
func (e *erasedReaders) besideWriter(in Inst, a LocalID, o ownerRef) (LocalID, bool) {
	for _, b := range in.Args {
		if b == a {
			continue
		}
		if hit, _ := e.bc.runHitVia(in, o, func(l LocalID, _ sem.EntityID) bool { return l != b }, storageReach); hit {
			return b, true
		}
	}
	return 0, false
}

// erasedParam reports an argument whose parameter type holds plain `fn` values (a variadic pack among them).
func (e *erasedReaders) erasedParam(in Inst, k int) bool {
	var t sem.TypeID
	switch {
	case in.Op == OpCallValue && k > 0:
		params, known := e.bc.calleeParams(e.f.Locals[in.Args[0]].Type)
		if !known || k-1 >= len(params) {
			return false
		}
		t = params[k-1]
	case in.Op == OpCall && in.Ent != 0 && e.p.R.Entity(in.Ent).Kind == sem.EntFn:
		info := e.p.R.Fn(in.Ent)
		if info.Recv != sem.RecvNone {
			k--
		}
		if k < 0 || k >= len(info.Params) {
			return false
		}
		t = e.p.R.Entity(info.Params[k]).Type
	default:
		return false
	}
	return e.bc.typeHolds(t, func(t sem.TypeID) bool { return e.p.R.Types.Kind(t) == sem.KFn })
}

func (e *erasedReaders) argNode(l LocalID, in Inst) syntax.NodeID {
	if d := e.defs[l]; d != nil && d.Node != 0 {
		return d.Node
	}
	return in.Node
}
