package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// checkRecvArgs rejects a `self` or `mut self` call, direct or through a witness, whose `&mut`
// argument names the receiver's place; checked at the call alone, so arguments may read it first.
func (c *borrowCheck) checkRecvArgs(in midir.Inst) {
	args := in.Args
	if in.Op == midir.OpCallValue && len(args) > 0 {
		args = args[1:]
	}
	if (in.Op != midir.OpCall && in.Op != midir.OpCallValue) || in.Ent == 0 || len(args) < 2 || c.p.R.Entity(in.Ent).Kind != sem.EntFn {
		return
	}
	var recv []ownerRef
	switch c.p.R.Fn(in.Ent).Recv {
	case sem.RecvMut:
		recv = c.recvOwners(args[0])
	case sem.RecvSelf:
		if root, path, exact, ok := c.projectionPlace(args[0]); ok {
			recv = []ownerRef{{owner: root, path: path, exact: exact}}
		}
	}
	if len(recv) == 0 {
		return
	}
	for _, a := range args[1:] {
		if a == args[0] {
			continue
		}
		node := in.Node
		if def := c.definition(a); def != nil && def.Node != 0 {
			node = def.Node
		}
		for _, o := range c.mutArgOwners(a) {
			for _, r := range recv {
				if o.owner == r.owner && ownquery.FieldPathsOverlap(o.path, r.path) {
					c.report(node, r.owner, "borrow-conflict", "`"+c.f.Locals[r.owner].Name+"` is exclusively borrowed and cannot be used here")
				}
			}
		}
	}
}

// recvOwners is what a receiver cell lends: the place it borrows, plus that place's own
// owners when the place is a reference.
func (c *borrowCheck) recvOwners(cell midir.LocalID) []ownerRef {
	def := c.definition(cell)
	if def == nil || def.Op != midir.OpNewCell {
		return nil
	}
	src := def.Args[0]
	def = c.definition(src)
	for def != nil && def.Op == midir.OpShare {
		src = def.Args[0]
		def = c.definition(src)
	}
	if def == nil || def.Op != midir.OpBorrow || def.Index != 1 {
		return nil
	}
	root, path, exact, ok := c.projectionPlace(def.Args[0])
	if !ok {
		return nil
	}
	out := []ownerRef{{owner: root, mut: true, path: path, exact: exact}}
	if b, ok := c.holders[root]; ok && b.hasOwner && len(path) == 0 {
		for _, o := range b.owners {
			// A tie this very call stores into the receiver starts after it.
			if !o.tied || c.tieReaches(root, o.owner, c.curIdx) {
				out = append(out, o)
			}
		}
	}
	return out
}

// mutArgOwners is the places a call argument lends exclusively: an `&mut` borrow's
// owners, or a `&mut` reference local passed on as itself.
func (c *borrowCheck) mutArgOwners(a midir.LocalID) []ownerRef {
	if !c.p.R.Types.RefMut(c.f.Locals[a].Type) {
		return nil
	}
	out := []ownerRef{{owner: a, mut: true, exact: true}}
	if root, path, exact, ok := c.projectionPlace(a); ok {
		out = append(out, ownerRef{owner: root, mut: true, path: path, exact: exact})
	}
	if b, ok := c.holders[a]; ok && b.hasOwner {
		out = append(out, b.owners...)
	}
	return out
}
