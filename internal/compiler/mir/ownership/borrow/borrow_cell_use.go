package borrow

import (
	"slices"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// capReach picks the capture modes through which a closure works on a local's own storage.
type capReach func(sem.CaptureMode) bool

func cellReach(m sem.CaptureMode) bool { return m == sem.CapCell }

// storageReach adds a lent borrow, which reads and writes the local's storage in place as a cell capture does.
func storageReach(m sem.CaptureMode) bool { return m == sem.CapCell || m == sem.CapBorrow }

// cellUses lists the places g reaches in cell: field reads and stores through it, `&` / `&mut` of a part of it,
// what a nested closure capturing it reaches, and the whole value when the cell itself is replaced or handed on.
func cellUses(p *ownquery.Input, g *midir.Func, cell midir.LocalID, depth int) []cellUse {
	if depth > 16 {
		return []cellUse{{nil, true}}
	}
	defs, chained := mirflow.SoleDefs(g), chainedTemps(g)
	var out []cellUse
	add := func(path ownquery.Path, w bool) {
		if !slices.ContainsFunc(out, func(u cellUse) bool { return u.writes == w && slices.Equal(u.path, path) }) {
			out = append(out, cellUse{path, w})
		}
	}
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op != midir.OpClosure {
				if u, ok := cellAccess(g, defs, chained, cell, in); ok {
					add(u.path, u.writes)
				}
				continue
			}
			for j, a := range in.Args {
				if h := p.ByEnt[in.Ent]; a == cell && (h == nil || j >= len(h.Params)) {
					add(nil, true)
				} else if a == cell {
					for _, u := range cellUses(p, h, h.Params[j], depth+1) {
						add(u.path, u.writes)
					}
				}
			}
		}
	}
	return out
}

// cellAccess is the place one instruction other than a closure's construction reaches in cell, if any.
func cellAccess(g *midir.Func, defs map[midir.LocalID]*midir.Inst, chained map[midir.LocalID]bool, cell midir.LocalID, in midir.Inst) (cellUse, bool) {
	place := func(l midir.LocalID) (ownquery.Path, bool, bool) {
		root, path, exact, ok := storageContentPlace(g, defs, l)
		return path, exact, ok && root == cell
	}
	switch in.Op {
	case midir.OpDrop:
		return cellUse{}, false
	case midir.OpCellGet, midir.OpField, midir.OpPayload, midir.OpIndex, midir.OpUnbox:
		path, _, ok := place(in.Dst)
		return cellUse{path, false}, ok && !chained[in.Dst]
	case midir.OpSetField, midir.OpFieldMove:
		if path, exact, ok := place(in.Args[0]); ok {
			if exact {
				path = append(slices.Clone(path), in.Index)
			}
			return cellUse{path, true}, true
		}
	case midir.OpSetIndex:
		if path, _, ok := place(in.Args[0]); ok {
			return cellUse{path, true}, true
		}
	case midir.OpBorrow:
		if path, _, ok := place(in.Args[0]); ok {
			return cellUse{path, in.Index == 1}, true
		}
	}
	return cellUse{nil, true}, slices.Contains(in.Args, cell)
}

// An index or unbox cuts the field chain because the selected element has no fixed field path.
func storageContentPlace(g *midir.Func, defs map[midir.LocalID]*midir.Inst, l midir.LocalID) (root midir.LocalID, path ownquery.Path, exact, ok bool) {
	exact = true
	for g.Locals[l].Ent == 0 {
		d := defs[l]
		if d == nil || len(d.Args) == 0 {
			return 0, nil, false, false
		}
		switch d.Op {
		case midir.OpCellGet:
			return d.Args[0], path, exact, true
		case midir.OpField, midir.OpPayload, midir.OpIndex, midir.OpUnbox:
			path, exact = path.Under(d, exact, nil)
		case midir.OpCopy, midir.OpAlias:
		default:
			return 0, nil, false, false
		}
		l = d.Args[0]
	}
	return l, path, exact, true
}

// chainedTemps are g's temporaries used only as the base of a further projection, store or borrow.
func chainedTemps(g *midir.Func) map[midir.LocalID]bool {
	total, chained := map[midir.LocalID]int{}, map[midir.LocalID]int{}
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			for _, a := range in.Args {
				total[a]++
			}
			switch in.Op {
			case midir.OpField, midir.OpSetField, midir.OpIndex, midir.OpSetIndex, midir.OpPayload, midir.OpUnbox, midir.OpBorrow, midir.OpFieldMove:
				chained[in.Args[0]]++
			}
		}
		for _, a := range blk.Term.Args {
			total[a]++
		}
	}
	out := map[midir.LocalID]bool{}
	for l, n := range chained {
		out[l] = n == total[l]
	}
	return out
}
