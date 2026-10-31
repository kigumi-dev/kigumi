package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
)

type capturedPlace struct {
	root midir.LocalID
	path ownquery.Path
}

func checkCapturedFieldMoves(p *ownquery.Input, f *midir.Func) {
	defs := map[midir.LocalID]*midir.Inst{}
	multi := map[midir.LocalID]bool{}
	for bi := range f.Blocks {
		for ii := range f.Blocks[bi].Insts {
			in := &f.Blocks[bi].Insts[ii]
			if !in.Op.Yields() {
				continue
			}
			if defs[in.Dst] != nil {
				multi[in.Dst] = true
			}
			defs[in.Dst] = in
		}
	}
	place := func(l midir.LocalID) capturedPlace {
		var path ownquery.Path
		for !multi[l] && defs[l] != nil {
			d := defs[l]
			switch {
			case d.Op == midir.OpField:
				path, _ = path.Under(d, true, nil)
			case d.Op == midir.OpCopy && f.Locals[l].Ent == 0:
			default:
				return capturedPlace{l, path}
			}
			l = d.Args[0]
		}
		return capturedPlace{l, path}
	}
	caps := capturedHolders(p, f, place)
	if len(caps) == 0 {
		return
	}
	liveOut := capturedLiveness(f, caps, nil)
	for bi := range f.Blocks {
		blk := &f.Blocks[bi]
		live := map[midir.LocalID]bool{}
		for l := range liveOut[bi] {
			live[l] = true
		}
		markCapturedUses(blk.Term.Args, caps, live)
		for i := len(blk.Insts) - 1; i >= 0; i-- {
			in := &blk.Insts[i]
			if in.Op == midir.OpFieldMove {
				moved := place(in.Args[0])
				moved.path = append(moved.path, in.Index)
				if sharedBy(moved, caps, live) {
					reportCapturedMove(p, f, in.Node, moved.root)
				}
			}
			if in.Op == midir.OpDrop {
				continue
			}
			delete(live, in.Dst)
			markCapturedUses(in.Args, caps, live)
		}
	}
}

func sharedBy(moved capturedPlace, caps map[midir.LocalID][]capturedPlace, live map[midir.LocalID]bool) bool {
	_, ok := sharerOf(moved, caps, live)
	return ok
}

// sharerOf is a live holder of a place overlapping moved; the lowest id keeps the choice stable.
func sharerOf(moved capturedPlace, caps map[midir.LocalID][]capturedPlace, live map[midir.LocalID]bool) (midir.LocalID, bool) {
	var out midir.LocalID
	found := false
	for l := range live {
		for _, cp := range caps[l] {
			if cp.root == moved.root && ownquery.FieldPathsOverlap(cp.path, moved.path) && (!found || l < out) {
				out, found = l, true
			}
		}
	}
	return out, found
}

func markCapturedUses(args []midir.LocalID, caps map[midir.LocalID][]capturedPlace, live map[midir.LocalID]bool) {
	for _, a := range args {
		if _, ok := caps[a]; ok {
			live[a] = true
		}
	}
}

func reportCapturedMove(p *ownquery.Input, f *midir.Func, n syntax.NodeID, root midir.LocalID) {
	if n == 0 {
		return
	}
	t := p.R.Tree(f.File)
	msg := "`" + f.Locals[root].Name + "` is shared with a closure that runs later, so a field cannot be moved out of it here"
	f.Diags = append(f.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Code: "move-while-borrowed", Msg: msg})
}
