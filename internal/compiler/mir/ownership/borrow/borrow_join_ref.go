package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// joinedRefBorrow is a borrow under a temporary joining references, as in `&(if k { &a } else { &b }).f`:
// it borrows what every arm borrows, refreshed as the arms' holders grow.
func (c *borrowCheck) joinedRefBorrow(in midir.Inst) (*borrowInfo, bool) {
	root, path, exact, ok := c.joinedRefPlace(in.Args[0])
	mut := in.Index == 1
	if !ok || c.throughRef(root, path, exact, mut) == nil {
		return nil, false
	}
	b := &borrowInfo{hasOwner: true, node: in.Node}
	site := reborrowSite{b, root, path, exact, mut}
	c.reborrows = append(c.reborrows, site)
	site.refresh(c)
	return b, true
}

func (c *borrowCheck) joinedRefPlace(l midir.LocalID) (midir.LocalID, ownquery.Path, bool, bool) {
	var path ownquery.Path
	exact := true
	for c.f.Locals[l].Ent == 0 {
		d := c.definition(l)
		if d == nil {
			return l, path, exact, c.multiDefined(l) && c.p.R.Types.Kind(c.f.Locals[l].Type) == sem.KRef
		}
		switch d.Op {
		case midir.OpField, midir.OpPayload, midir.OpIndex, midir.OpUnbox:
			path, exact = path.Under(d, exact, nil)
		case midir.OpCopy:
		default:
			return 0, nil, false, false
		}
		l = d.Args[0]
	}
	return 0, nil, false, false
}
