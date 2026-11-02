package borrow

import (
	"cmp"
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// relayStore is a value a call may store; param is the unknown callee's `&'a T` parameter type it is passed to when 'a
// is a lifetime the enclosing function declares, and 0 otherwise.
type relayStore struct {
	flowArg
	param sem.TypeID
}

func (c *borrowCheck) callerLifetimeParam(sig sem.TypeID, k int) sem.TypeID {
	r := c.p.R
	if !r.Types.FnParamKeepsRef(sig, k) {
		return 0
	}
	p := r.Types.Node(sig).Args[k]
	lt := r.Types.Node(r.Types.RefLifetime(p))
	fn := c.f.Ent
	for fn != 0 && r.Entity(fn).Kind != sem.EntFn {
		fn = r.Entity(fn).Parent
	}
	if lt.Kind != sem.KParam || fn == 0 || !slices.Contains(r.Fn(fn).TypeParams, lt.Ent) {
		return 0
	}
	return p
}

func (c *borrowCheck) reportCallerLifetime(in midir.Inst, a midir.LocalID, b *borrowInfo, param sem.TypeID) {
	code, _ := sem.CodeByName("borrow-caller-lifetime")
	name := c.carriedName(b)
	msg, help := sem.FillCode(code, name, c.p.R.TypeString(param), c.p.R.TypeString(c.p.R.Types.RefLifetime(param)))
	c.reportHelped(in.Node, a, code.Num, msg, c.ownerBorrowSite(b), name+" is borrowed here", help)
}

// ownerBorrowSite is where the owner carriedName names is borrowed when that is a single place: a borrow joined from
// several sources keeps the node of only one of them.
func (c *borrowCheck) ownerBorrowSite(b *borrowInfo) syntax.NodeID {
	for _, o := range b.owners {
		if o.view {
			return b.node
		}
		if c.f.Locals[o.owner].Ent == 0 || c.f.Locals[o.owner].Name == "" {
			continue
		}
		var at syntax.NodeID
		for _, blk := range c.f.Blocks {
			for _, in := range blk.Insts {
				if in.Op == midir.OpBorrow && len(in.Args) > 0 && in.Args[0] == o.owner {
					if at != 0 {
						return b.node
					}
					at = in.Node
				}
			}
		}
		return cmp.Or(at, b.node)
	}
	return b.node
}
