package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/syntax"
)

type capturedPlace struct {
	root LocalID
	path []int
}

func checkCapturedFieldMoves(p *Program, f *Func) {
	defs := map[LocalID]*Inst{}
	multi := map[LocalID]bool{}
	for bi := range f.Blocks {
		for ii := range f.Blocks[bi].Insts {
			in := &f.Blocks[bi].Insts[ii]
			if in.Dst == 0 {
				continue
			}
			if defs[in.Dst] != nil {
				multi[in.Dst] = true
			}
			defs[in.Dst] = in
		}
	}
	place := func(l LocalID) capturedPlace {
		var path []int
		for !multi[l] && defs[l] != nil {
			d := defs[l]
			switch {
			case d.Op == OpField:
				path = append([]int{d.Index}, path...)
			case d.Op == OpCopy && f.Locals[l].Ent == 0:
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
			if in.Op == OpFieldMove {
				moved := place(in.Args[0])
				moved.path = append(moved.path, in.Index)
				if sharedBy(moved, caps, live) {
					reportCapturedMove(p, f, in.Node, moved.root)
				}
			}
			if in.Op == OpDrop {
				continue
			}
			delete(live, in.Dst)
			markCapturedUses(in.Args, caps, live)
		}
	}
}

func sharedBy(moved capturedPlace, caps map[LocalID][]capturedPlace, live map[LocalID]bool) bool {
	for l := range live {
		for _, cp := range caps[l] {
			if cp.root == moved.root && fieldPathsOverlap(cp.path, moved.path, true, true) {
				return true
			}
		}
	}
	return false
}

func markCapturedUses(args []LocalID, caps map[LocalID][]capturedPlace, live map[LocalID]bool) {
	for _, a := range args {
		if _, ok := caps[a]; ok {
			live[a] = true
		}
	}
}

func reportCapturedMove(p *Program, f *Func, n syntax.NodeID, root LocalID) {
	if n == 0 {
		return
	}
	t := p.R.Tree(f.File)
	msg := "`" + f.Locals[root].Name + "` is shared with a closure that runs later, so a field cannot be moved out of it here"
	f.Diags = append(f.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Code: "move-while-borrowed", Msg: msg})
}
