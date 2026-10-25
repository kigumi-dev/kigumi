package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// capReach picks the capture modes through which a closure works on a local's own storage.
type capReach func(sem.CaptureMode) bool

func cellReach(m sem.CaptureMode) bool { return m == sem.CapCell }

// storageReach adds a lent borrow, which reads and writes the local's storage in place as a cell capture does.
func storageReach(m sem.CaptureMode) bool { return m == sem.CapCell || m == sem.CapBorrow }

// cellUses lists the places g reaches in cell: field reads and stores through it, `&` / `&mut` of a part of it,
// what a nested closure capturing it reaches, and the whole value when the cell itself is replaced or handed on.
func cellUses(p *Program, g *Func, cell LocalID, depth int) []cellUse {
	if depth > 16 {
		return []cellUse{{nil, true}}
	}
	defs, chained := soleDefs(g), chainedTemps(g)
	var out []cellUse
	add := func(path stepPath, w bool) {
		if !slices.ContainsFunc(out, func(u cellUse) bool { return u.writes == w && slices.Equal(u.path, path) }) {
			out = append(out, cellUse{path, w})
		}
	}
	place := func(l LocalID) (stepPath, bool, bool) {
		root, path, exact, ok := cellContentPlace(g, defs, l)
		return path, exact, ok && root == cell
	}
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			switch in.Op {
			case OpDrop:
				continue
			case OpCellGet, OpField, OpPayload, OpIndex, OpUnbox:
				if path, _, ok := place(in.Dst); ok && !chained[in.Dst] {
					add(path, false)
				}
				continue
			case OpSetField, OpFieldMove:
				if path, exact, ok := place(in.Args[0]); ok {
					if exact {
						path = append(slices.Clone(path), in.Index)
					}
					add(path, true)
					continue
				}
			case OpSetIndex:
				if path, _, ok := place(in.Args[0]); ok {
					add(path, true)
					continue
				}
			case OpBorrow:
				if path, _, ok := place(in.Args[0]); ok {
					add(path, in.Index == 1)
					continue
				}
			case OpClosure:
				for j, a := range in.Args {
					if h := p.ByEnt[in.Ent]; a == cell && (h == nil || j >= len(h.Params)) {
						add(nil, true)
					} else if a == cell {
						for _, u := range cellUses(p, h, h.Params[j], depth+1) {
							add(u.path, u.writes)
						}
					}
				}
				continue
			}
			if slices.Contains(in.Args, cell) {
				add(nil, true)
			}
		}
	}
	return out
}

// cellContentPlace follows a temporary back to the cell whose content it reads, with the field chain; an index or unbox
// step cuts the chain there, as walkPlace does.
func cellContentPlace(g *Func, defs map[LocalID]*Inst, l LocalID) (cell LocalID, path stepPath, exact, ok bool) {
	exact = true
	for g.Locals[l].Ent == 0 {
		d := defs[l]
		if d == nil || len(d.Args) == 0 {
			return 0, nil, false, false
		}
		switch d.Op {
		case OpCellGet:
			return d.Args[0], path, exact, true
		case OpField, OpPayload, OpIndex, OpUnbox:
			path, exact = path.under(d, exact, nil)
		case OpCopy, OpAlias:
		default:
			return 0, nil, false, false
		}
		l = d.Args[0]
	}
	return 0, nil, false, false
}

// chainedTemps are g's temporaries used only as the base of a further projection, store or borrow.
func chainedTemps(g *Func) map[LocalID]bool {
	total, chained := map[LocalID]int{}, map[LocalID]int{}
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			for _, a := range in.Args {
				total[a]++
			}
			switch in.Op {
			case OpField, OpSetField, OpIndex, OpSetIndex, OpPayload, OpUnbox, OpBorrow, OpFieldMove:
				chained[in.Args[0]]++
			}
		}
		for _, a := range blk.Term.Args {
			total[a]++
		}
	}
	out := map[LocalID]bool{}
	for l, n := range chained {
		out[l] = n == total[l]
	}
	return out
}
