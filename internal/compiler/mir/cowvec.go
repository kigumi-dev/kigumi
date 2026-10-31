package mir

import (
	"slices"
	"sync/atomic"

	midir "kigumi/internal/compiler/mir/ir"
)

const (
	cowLeafSize = 64
	cowNodeSize = 64
	cowSpan     = cowLeafSize * cowNodeSize
)

var cowOwners atomic.Int32

type cowLeaf[T comparable] struct {
	owner int32
	v     [cowLeafSize]T
}

type cowNode[T comparable] struct {
	owner int32
	leaf  [cowNodeSize]*cowLeaf[T]
}

// cowVec maps each local to a value, zero when unset; clones share nodes and the first write copies one.
type cowVec[T comparable] struct {
	top []*cowNode[T]
	id  int32
}

func (v *cowVec[T]) get(l midir.LocalID) T {
	var zero T
	t := int(l) / cowSpan
	if t >= len(v.top) || v.top[t] == nil {
		return zero
	}
	lf := v.top[t].leaf[int(l)/cowLeafSize%cowNodeSize]
	if lf == nil {
		return zero
	}
	return lf.v[int(l)%cowLeafSize]
}

func (v *cowVec[T]) set(l midir.LocalID, x T) {
	if v.get(l) == x {
		return
	}
	if v.id == 0 {
		v.id = cowOwners.Add(1)
	}
	t, c := int(l)/cowSpan, int(l)/cowLeafSize%cowNodeSize
	for t >= len(v.top) {
		v.top = append(v.top, nil)
	}
	switch nd := v.top[t]; {
	case nd == nil:
		v.top[t] = &cowNode[T]{owner: v.id}
	case nd.owner != v.id:
		cp := *nd
		cp.owner = v.id
		v.top[t] = &cp
	}
	nd := v.top[t]
	switch lf := nd.leaf[c]; {
	case lf == nil:
		nd.leaf[c] = &cowLeaf[T]{owner: v.id}
	case lf.owner != v.id:
		cp := *lf
		cp.owner = v.id
		nd.leaf[c] = &cp
	}
	nd.leaf[c].v[int(l)%cowLeafSize] = x
}

func (v *cowVec[T]) clone() cowVec[T] {
	v.id = cowOwners.Add(1)
	return cowVec[T]{top: slices.Clone(v.top), id: cowOwners.Add(1)}
}

// join replaces each entry with f(entry, other's entry) and reports a change; f(x, x) must be x.
func (v *cowVec[T]) join(o *cowVec[T], f func(cur, other T) T) bool { return v.joinNodes(o, f, false) }

// joinIdent is join for an f with zero as identity on either side: what o lacks is skipped, what v lacks is taken.
func (v *cowVec[T]) joinIdent(o *cowVec[T], f func(cur, other T) T) bool {
	return v.joinNodes(o, f, true)
}

// joinNodes shares o's leaves and nodes where the result equals them, so o takes a fresh id as clone does.
func (v *cowVec[T]) joinNodes(o *cowVec[T], f func(cur, other T) T, ident bool) bool {
	if v.id == 0 {
		v.id = cowOwners.Add(1)
	}
	n := len(o.top)
	if !ident {
		n = max(n, len(v.top))
	}
	changed := false
	for t := range n {
		vn, on := v.nodeAt(t), o.nodeAt(t)
		if vn == on || ident && on == nil {
			continue
		}
		out, dirty := v.joinLeaves(vn, on, f, ident)
		if !dirty {
			continue
		}
		changed = true
		for t >= len(v.top) {
			v.top = append(v.top, nil)
		}
		switch {
		case on != nil && out == on.leaf:
			v.top[t] = on
		case vn != nil && vn.owner == v.id:
			vn.leaf = out
		default:
			v.top[t] = &cowNode[T]{owner: v.id, leaf: out}
		}
	}
	if changed {
		o.id = cowOwners.Add(1)
	}
	return changed
}

func (v *cowVec[T]) joinLeaves(vn, on *cowNode[T], f func(cur, other T) T, ident bool) (out [cowNodeSize]*cowLeaf[T], dirty bool) {
	if vn != nil {
		out = vn.leaf
	}
	for c := range cowNodeSize {
		vl, ol := out[c], on.leafOf(c)
		if vl == ol {
			continue
		}
		cur, other := vl.values(), ol.values()
		j := cur
		switch {
		case cur == other || ident && other == [cowLeafSize]T{}:
		case ident && cur == [cowLeafSize]T{}:
			j = other
		default:
			for i, x := range other {
				j[i] = f(cur[i], x)
			}
		}
		switch {
		case ol != nil && j == other:
			out[c] = ol
		case j == cur:
		case vl != nil && vl.owner == v.id:
			vl.v = j
		default:
			out[c] = &cowLeaf[T]{owner: v.id, v: j}
		}
		dirty = dirty || j != cur
	}
	return out, dirty
}

func (v *cowVec[T]) nodeAt(t int) *cowNode[T] {
	if t >= len(v.top) {
		return nil
	}
	return v.top[t]
}

func (n *cowNode[T]) leafOf(c int) *cowLeaf[T] {
	if n == nil {
		return nil
	}
	return n.leaf[c]
}

func (l *cowLeaf[T]) values() [cowLeafSize]T {
	if l == nil {
		return [cowLeafSize]T{}
	}
	return l.v
}
