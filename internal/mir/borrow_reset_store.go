package mir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// replacesHolder reports whether in stores into the named holder's only field that may carry a borrow, a bare
// reference, so the store ends every borrow the holder had.
func (c *borrowCheck) replacesHolder(in Inst, holder LocalID) bool {
	if in.Op != OpSetField || len(in.Args) == 0 || in.Args[0] != holder || c.f.Locals[holder].Ent == 0 {
		return false
	}
	tt := c.p.R.Types
	t := c.f.Locals[holder].Type
	if tt.Kind(t) != sem.KNamed {
		return false
	}
	decl := c.p.R.TypeDecl(tt.Node(t).Ent)
	if decl == nil || decl.Form != sem.FormRecord {
		return false
	}
	for _, p := range decl.Params {
		if !c.p.R.TypeParam(p).IsLifetime {
			return false
		}
	}
	for _, fid := range decl.Fields {
		f := c.p.R.Field(fid)
		if f.Index == in.Index {
			if f.Lifetime == 0 || tt.Kind(f.Type) != sem.KRef {
				return false
			}
		} else if f.Lifetime != 0 || c.resultMayCarryLifetime(f.Type) || !c.p.R.IsCopy(f.Type) || len(tt.Params(f.Type)) != 0 {
			return false
		}
	}
	return true
}

// storesOnlyReplace reports whether every store or carried value written into holder is a replacing field store.
func (c *borrowCheck) storesOnlyReplace(holder LocalID) bool {
	for _, blk := range c.f.Blocks {
		for _, in := range blk.Insts {
			for _, fl := range c.storeFlows(in) {
				if root, _, _, ok := c.storeRoot(fl); ok && root == holder && !c.replacesHolder(in, holder) {
					return false
				}
			}
			targets, _ := c.carryFlows(in)
			for _, t := range targets {
				if root, _, _, ok := c.storeRoot(storeFlow{target: t, ref: true}); ok && root == holder {
					return false
				}
			}
		}
	}
	return true
}

// storedAt stamps the owners a store brings into a holder with the store's borrow, so a conflict's note points there.
func storedAt(b *borrowInfo) *borrowInfo {
	out := *b
	out.owners = make([]ownerRef, len(b.owners))
	for i, o := range b.owners {
		if o.node == 0 {
			o.node = b.node
		}
		out.owners[i] = o
	}
	return &out
}

func (o ownerRef) at(b *borrowInfo) syntax.NodeID {
	if o.node != 0 {
		return o.node
	}
	return b.node
}
