package ownership

import (
	"strconv"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

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
	a.moves = append(a.moves, MoveEvent{At: a.pos, Root: root, Node: in.Node})
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
