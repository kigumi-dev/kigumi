package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// replacesHolder reports whether in stores into the named holder's only field that may carry a borrow, so the store
// ends every borrow the holder had.
func (c *borrowCheck) replacesHolder(in midir.Inst, holder midir.LocalID) bool {
	return in.Op == midir.OpSetField && len(in.Args) > 0 && in.Args[0] == holder && c.f.Locals[holder].Ent != 0 && c.analysis().replacingField(c.f.Locals[holder].Type, in.Index)
}

// replacingField reports whether field index of record type t is a bare reference and its only field that may carry a borrow.
func (p *borrowAnalysis) replacingField(t sem.TypeID, index int) bool {
	tt := p.R.Types
	if tt.Kind(t) != sem.KNamed {
		return false
	}
	decl := p.R.TypeDecl(tt.Node(t).Ent)
	if decl == nil || decl.Form != sem.FormRecord {
		return false
	}
	for _, prm := range decl.Params {
		if !p.R.TypeParam(prm).IsLifetime {
			return false
		}
	}
	for _, fid := range decl.Fields {
		f := p.R.Field(fid)
		if f.Index == index {
			if f.Lifetime == 0 || tt.Kind(f.Type) != sem.KRef {
				return false
			}
		} else if f.Lifetime != 0 || tt.Kind(f.Type) == sem.KRef || !p.R.IsCopy(f.Type) || len(tt.Params(f.Type)) != 0 {
			return false
		}
	}
	return true
}

// storesOnlyReplace reports whether every store or carried value written into holder is a replacing field store;
// it is asked once holders are final, so one pass over the stores answers it for every holder.
func (c *borrowCheck) storesOnlyReplace(holder midir.LocalID) bool {
	if c.resets.stored == nil {
		c.resets.stored = map[midir.LocalID]bool{}
		for _, blk := range c.f.Blocks {
			for _, in := range blk.Insts {
				for _, fl := range c.storeFlows(in) {
					for _, r := range c.storeDests(fl) {
						c.resets.stored[r] = c.resets.stored[r] || !c.replacesHolder(in, r) && !c.callResets(in, fl, r)
					}
				}
				targets, _ := c.carryFlows(in)
				for _, fl := range targets {
					for _, r := range c.storeDests(fl) {
						c.resets.stored[r] = true
					}
				}
			}
		}
	}
	return !c.resets.stored[holder]
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
