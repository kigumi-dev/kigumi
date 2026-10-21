package mir

import (
	"slices"
	"sync/atomic"
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

func (v *cowVec[T]) get(l LocalID) T {
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

func (v *cowVec[T]) set(l LocalID, x T) {
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

// join replaces each entry with f(entry, other's entry) and reports a change; leaves both share are skipped.
func (v *cowVec[T]) join(o *cowVec[T], f func(cur, other T) T) bool {
	changed := false
	var empty cowLeaf[T]
	for t := range max(len(v.top), len(o.top)) {
		for c := range cowNodeSize {
			vl, ol := v.leafAt(t, c), o.leafAt(t, c)
			if vl == ol {
				continue
			}
			if ol == nil {
				ol = &empty
			}
			for i, x := range ol.v {
				l := LocalID(t*cowSpan + c*cowLeafSize + i)
				cur := v.get(l)
				if j := f(cur, x); j != cur {
					v.set(l, j)
					changed = true
				}
			}
		}
	}
	return changed
}

func (v *cowVec[T]) leafAt(t, c int) *cowLeaf[T] {
	if t >= len(v.top) || v.top[t] == nil {
		return nil
	}
	return v.top[t].leaf[c]
}

// each visits the entries that are set.
func (v *cowVec[T]) each(fn func(l LocalID, x T)) {
	var zero T
	for t, nd := range v.top {
		if nd == nil {
			continue
		}
		for c, lf := range nd.leaf {
			if lf == nil {
				continue
			}
			for i, x := range lf.v {
				if x != zero {
					fn(LocalID(t*cowSpan+c*cowLeafSize+i), x)
				}
			}
		}
	}
}
