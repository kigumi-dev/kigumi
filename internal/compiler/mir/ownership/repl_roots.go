package ownership

import midir "kigumi/internal/compiler/mir/ir"

// MovedRoots maps each moved place to the local that roots it. The REPL uses
// this to remove session bindings whose whole value, or a field of it, was
// consumed by a cell.
func MovedRoots(p *Input, f *midir.Func, moves []MoveEvent) map[midir.LocalID]bool {
	if len(moves) == 0 {
		return nil
	}
	pl := findPlaces(p, f)
	out := map[midir.LocalID]bool{}
	for _, m := range moves {
		l := m.Root
		for {
			par, ok := pl.parent[l]
			if !ok {
				break
			}
			l = par
		}
		out[l] = true
	}
	return out
}
