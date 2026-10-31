package mir

import (
	"strconv"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// places gives every chain of field reads rooted at a named local one identity,
// so that two reads of the same field are one place to ownership analysis
// rather than two unrelated temporaries.
type places struct {
	key  map[midir.LocalID]string
	rep  map[string]midir.LocalID
	name map[midir.LocalID]string
	// children lists a root's own direct field-place representatives, by
	// field index, so own.go can tell which of its fields are still valid.
	children map[midir.LocalID]map[int]midir.LocalID
	parent   map[midir.LocalID]midir.LocalID
	// cellRoot maps a read of a lent receiver's write-back cell back to the
	// receiver, so `self.a` keys the same place a plain receiver's would.
	cellRoot map[midir.LocalID]midir.LocalID
}

// findPlaces keys each projection temporary and elects the first temporary of a
// key as its representative. Blocks are revisited until stable because a chain
// can cross a back edge.
func findPlaces(p *Program, f *midir.Func) *places {
	pl := &places{key: map[midir.LocalID]string{}, rep: map[string]midir.LocalID{}, name: map[midir.LocalID]string{},
		children: map[midir.LocalID]map[int]midir.LocalID{}, parent: map[midir.LocalID]midir.LocalID{}, cellRoot: map[midir.LocalID]midir.LocalID{}}
	// checkBorrowedMoves already rejects every move out of a closure's
	// captured receiver; keying places there reports the same move twice.
	lentOK := p.R.Entity(f.Ent).Kind != sem.EntClosure
	for bi := range f.Blocks {
		for _, in := range f.Blocks[bi].Insts {
			if in.Op == midir.OpCellGet && (lentOK && f.Locals[in.Args[0]].Borrowed || trackedCell(p, f.Locals[in.Args[0]])) {
				pl.cellRoot[in.Dst] = in.Args[0]
			}
		}
	}
	t := p.R.Tree(f.File)
	for changed := true; changed; {
		changed = false
		for bi := range f.Blocks {
			for _, in := range f.Blocks[bi].Insts {
				if !pl.projects(t, in) || pl.key[in.Dst] != "" {
					continue
				}
				base, ok := pl.base(f, in)
				if !ok {
					continue
				}
				key := base + "." + pl.step(in)
				pl.key[in.Dst] = key
				if _, seen := pl.rep[key]; !seen {
					pl.rep[key] = in.Dst
					pl.name[in.Dst] = pl.placeName(p, f, t, in)
				}
				changed = true
			}
		}
	}
	for bi := range f.Blocks {
		for _, in := range f.Blocks[bi].Insts {
			if !pl.projects(t, in) || pl.canon(in.Dst) != in.Dst {
				continue
			}
			root := pl.canon(in.Args[0])
			if pl.children[root] == nil {
				pl.children[root] = map[int]midir.LocalID{}
			}
			pl.children[root][in.Index] = in.Dst
			pl.parent[in.Dst] = root
		}
	}
	return pl
}

// base names the container a field instruction reads from: another projection,
// or a named local. Temporaries of unknown provenance have no place.
func (pl *places) base(f *midir.Func, in midir.Inst) (string, bool) {
	if k := pl.key[in.Args[0]]; k != "" {
		return k, true
	}
	a := pl.canon(in.Args[0])
	if f.Locals[a].Ent == 0 {
		return "", false
	}
	return strconv.Itoa(int(a)), true
}

func (pl *places) isPlace(l midir.LocalID) bool { return pl.key[l] != "" }

func (pl *places) canon(l midir.LocalID) midir.LocalID {
	if k := pl.key[l]; k != "" {
		return pl.rep[k]
	}
	if r, ok := pl.cellRoot[l]; ok {
		return r
	}
	return l
}

// stored names the place a field store writes back, which owns a value again.
func (pl *places) stored(f *midir.Func, in midir.Inst) (midir.LocalID, bool) {
	base, ok := pl.base(f, in)
	if !ok {
		return 0, false
	}
	l, ok := pl.rep[base+"."+strconv.Itoa(in.Index)]
	return l, ok
}

// consumesArg reports whether argument i hands ownership over. It mirrors the
// backends' lending rule: a `self`/`mut self` receiver, the callee of an
// indirect call and every argument of a runtime primitive are lent, not
// moved, except `await`'s future (it runs the future's body and discards it)
// and a resource's own registered `drop`, which always consumes `self`.
func (p *Program) consumesArg(in *midir.Inst, i int) bool {
	switch in.Op {
	case midir.OpCall:
		e := p.R.Entity(in.Ent)
		if e.Kind != sem.EntFn {
			return false
		}
		info := p.R.Fn(in.Ent)
		if i == 0 && p.isOwnDrop(in.Ent, info) {
			return true
		}
		if info.Body == 0 && info.Abi == "" && e.Flags&sem.EfStd != 0 {
			return false
		}
		// C code releases nothing: every argument of a foreign call is lent.
		if info.Abi != "" || info.Naked {
			return false
		}
		// A trailing hidden provider argument forwards a lease; it never
		// changes hands.
		if i >= len(in.Args)-ctxArgCount(info) {
			return false
		}
		if g := p.ByEnt[in.Ent]; g != nil && i < len(g.Params) && g.Locals[g.Params[i]].Borrowed {
			return false
		}
		return i != 0 || info.Recv == sem.RecvNone || info.Recv == sem.RecvMove
	case midir.OpCallValue:
		// A witness call names its requirement: a `self` / `mut self`
		// receiver is lent, as in a direct method call.
		if in.Ent != 0 && i == 1 {
			recv := p.R.Fn(in.Ent).Recv
			return recv == sem.RecvNone || recv == sem.RecvMove
		}
		return i != 0
	case midir.OpAwait:
		return i == 0
	}
	return false
}

// ctxArgCount is how many trailing hidden provider arguments a call to info
// carries, the only ones with an actual parameter slot.
func ctxArgCount(info *sem.FnInfo) int {
	n := 0
	for _, k := range info.CtxRequires {
		if info.CtxReal[k] {
			n++
		}
	}
	return n
}

func (pl *places) resetFields(root midir.LocalID, st *dstate) {
	for _, rep := range pl.children[root] {
		st.val.del(rep)
		st.field.del(rep)
		st.called.del(rep)
		st.at.del(rep)
		st.callAt.del(rep)
		pl.resetFields(rep, st)
	}
}
