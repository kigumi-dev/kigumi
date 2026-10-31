package mir

import (
	"encoding/binary"
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// argContents is what a reference argument's referent holds, with what the places it keeps a `&mut` of hold, since
// whoever gets the reference may take their elements out.
func (c *borrowCheck) argContents(a midir.LocalID) (*borrowInfo, bool) {
	if r, ok := c.contentsMemo[a]; ok {
		return r.b, r.ok
	}
	b, ok := c.deepContents(a)
	if c.contentsMemo != nil {
		c.contentsMemo[a] = contentsResult{b, ok}
	}
	return b, ok
}

// contentsResult memoizes argContents for one round of collect (or one instruction's checks); a round that changes a
// holder runs another, which recomputes it.
type contentsResult struct {
	b  *borrowInfo
	ok bool
}

func (c *borrowCheck) deepContents(a midir.LocalID) (*borrowInfo, bool) {
	b, ok := c.referentContents(a)
	if !ok || b == nil || !b.hasOwner {
		return b, ok
	}
	return c.withHeld(b), true
}

// withHeld adds to b what the places it keeps a `&mut` of hold, and so on through their `&mut`s.
func (c *borrowCheck) withHeld(b *borrowInfo) *borrowInfo {
	var stack []midir.LocalID
	for _, o := range b.owners {
		if o.mut {
			stack = append(stack, o.owner)
		}
	}
	var extra []ownerRef
	have := map[ownerKey]bool{}
	for _, o := range b.owners {
		have[keyOf(o)] = true
	}
	for seen := map[midir.LocalID]bool{}; len(stack) > 0; {
		u := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		h, ok := c.holders[u]
		if seen[u] || !ok || !h.hasOwner {
			continue
		}
		seen[u] = true
		keeps := c.keepsBorrows(u)
		for _, o := range h.owners {
			if o.mut && !seen[o.owner] {
				stack = append(stack, o.owner)
			}
			if k := keyOf(o); keeps && !have[k] {
				have[k] = true
				extra = append(extra, o)
			}
		}
	}
	if len(extra) == 0 {
		return b
	}
	out := *b
	out.owners = append(slices.Clone(b.owners), extra...)
	return &out
}

// ownerKey is what ownerRef.same compares, with the path flattened so it can key a map.
type ownerKey struct {
	owner                                            midir.LocalID
	mut, exact, view, carried, caller, through, tied bool
	lifetime                                         sem.EntityID
	path                                             string
}

func keyOf(o ownerRef) ownerKey {
	var path []byte
	for _, s := range o.path {
		path = binary.AppendVarint(path, int64(s))
	}
	return ownerKey{o.owner, o.mut, o.exact, o.view, o.carried, o.caller, o.through, o.tied, o.lifetime, string(path)}
}

// A reborrow of a reference read out of a holder stands for what that reference points at.
func (c *borrowCheck) referentContents(a midir.LocalID) (*borrowInfo, bool) {
	for {
		if b, ok := c.paramContents(a); ok {
			return b, true
		}
		if c.f.Locals[a].Ent != 0 {
			break
		}
		in := c.definition(a)
		if in == nil || len(in.Args) == 0 {
			break
		}
		if in.Op == midir.OpBorrow {
			if refs := c.referents(in.Args[0]); len(refs) > 0 {
				return c.ownersContents(refs)
			}
			root, _, _, ok := c.projectionPlace(in.Args[0])
			if !ok {
				break
			}
			if b, ok := c.paramContents(root); ok {
				return b, true
			}
			b, ok := c.holders[root]
			return b, ok
		}
		if in.Op != midir.OpNewCell && in.Op != midir.OpShare && in.Op != midir.OpCopy && in.Op != midir.OpMove && in.Op != midir.OpAlias {
			break
		}
		a = in.Args[0]
	}
	b, ok := c.holders[a]
	return b, ok
}

func (c *borrowCheck) ownersContents(refs []ownerRef) (*borrowInfo, bool) {
	out := &borrowInfo{empty: true}
	for _, r := range refs {
		b, ok := c.paramContents(r.owner)
		if !ok {
			b, ok = c.holders[r.owner]
		}
		if ok {
			mergeInto(out, b)
		}
	}
	return out, !out.empty
}
